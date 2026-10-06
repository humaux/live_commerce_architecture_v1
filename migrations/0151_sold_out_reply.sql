-- Purpose: W3-04B sold-out automatic private reply: when the claim that created a bundle targets a sold-out offer, the one automatic private reply of the comment is a short "sold out" text (no claim link) instead of a link that opens onto no stock.
-- Depends on: 0064/0128 (integration.plan_claim_reply, claim_reply_plannable, claims.check_meta_reply, mpr: key), 0116 (A6 availability formula), 0121/0144 (msgtemplates.fixed_templates), 0123 (live.console_marks), 0143 (claim_reply_plannable suspension patch),
--   roles commerce_integration_writer / commerce_claims_writer / commerce_inventory_writer / commerce_msgtemplates_writer.
-- Used by: internal/claimsintake (planReply, unchanged call shape), internal/integrations/metareply (sold_out_reply dispatch), internal/claims/sold_out_reply.go (settings), tests/foundation/sold_out_reply_test.go.
-- Invariants: one private reply per comment (mpr: key, first writer wins: a sold-out reply CONSUMES it); claim is always recorded; stock is never reserved; Graph UNKNOWN is never re-sent.
-- Status: REAL_PG gate (Meta Graph MOCK; LIVE NOT_RUN).
-- 0151 sold-out reply (R3 unit W3-04B; docs/delivery/units/w3-04b-sold-out-reply.md "Integrator ruling").
--
-- What changes
--   * msgtemplates.fixed_templates gains sold-out-reply/v1 (kind private_reply, not public-safe, placeholder {{product.name}}); the template_id CHECK is
--     re-declared with the UNION of every fixed id up to 0144 plus this one (the integrator unions it with parallel units).
--   * claims.sold_out_settings (per store, absent row = enabled + the fixed template) with the merchant definers claims.get_sold_out_reply /
--     claims.set_sold_out_reply (live:read / live:manage; compare-and-swap on version; the chosen template must be a private_reply template of the
--     store or the fixed one, <= 280 chars, single line, only {{product.name}} as placeholder).
--   * inventory.claim_sku_sold_out: the 0116 A6 rule (tracked AND sum(on_hand-reserved-allocated-unavailable) < claimed quantity) as a boolean-only
--     definer for the claim reply planner. integration.claim_sold_out_facts: private helper both planners share, so they cannot disagree.
--   * claim_reply_plannable: a sold-out claim of a store whose switch is OFF is an audited skip (claim_reply_skipped:sold_out_off): the comment's
--     reply budget stays unspent for the merchant. Patched in place (like 0143) so the suspension patch survives.
--   * plan_claim_reply: sold-out claim => meta.private_reply operation with message_type sold_out_reply, the rendered text FROZEN in the request
--     (<= 400 chars, template text + product name, no buyer data), no claims.links row (nothing to sign), audit claim_reply_sold_out. Same mpr: key,
--     same unique index. Signature, owner, EXECUTE unchanged. check_meta_reply skips the link proof for that message type.
--   * live.console_marks: a sold_out_reply operation is shown as kind out_of_stock ("沒貨已回覆") in the comment feed.
-- Limit (documented in the contract): the reply consumes the comment's single private reply; a restock cannot re-reply to the same comment.
-- A paused/inactive offer never reaches here (ingest rejects it as OFFER_INACTIVE and creates no bundle); the predicate still treats it as sold out.

-- ---------------------------------------------------------------------------------------
-- Fixed template.
-- ---------------------------------------------------------------------------------------
ALTER TABLE msgtemplates.fixed_templates DROP CONSTRAINT fixed_templates_template_id_check;
ALTER TABLE msgtemplates.fixed_templates ADD CONSTRAINT fixed_templates_template_id_check
    CHECK (template_id IN ('order-pay-link/v1', 'offer-recommend/v1', 'checkout-reminder/v1', 'sold-out-reply/v1'));
INSERT INTO msgtemplates.fixed_templates(template_id, version, name, body, kinds, public_safe) VALUES
 ('sold-out-reply/v1', 1, '沒貨回覆', '抱歉，{{product.name}} 已售完，補貨時會在直播中通知，請留意直播。', ARRAY['private_reply'], false)
ON CONFLICT (template_id, version) DO NOTHING;

-- Body of a template usable as the sold-out reply (NULL = not usable): fixed or this store's, kind private_reply, one line of <= 280 characters,
-- {{product.name}} the only placeholder. Used by the setter (validation) and plan_claim_reply (rendering); versions are append-only, so a
-- body that was valid when chosen stays valid.
CREATE FUNCTION msgtemplates.sold_out_body(p_tenant uuid, p_store uuid, p_template_id text, p_version bigint) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT x.body FROM (
        SELECT f.body, f.kinds FROM msgtemplates.fixed_templates f WHERE f.template_id = p_template_id AND f.version = p_version
        UNION ALL
        SELECT t.body, t.kinds FROM msgtemplates.templates t
         WHERE t.tenant_id = p_tenant AND t.store_id = p_store AND t.template_id = p_template_id AND t.version = p_version
    ) x
    WHERE x.kinds @> ARRAY['private_reply']::text[]
      AND char_length(x.body) BETWEEN 1 AND 280
      AND x.body !~ '[[:cntrl:]]'
      AND regexp_replace(x.body, '\{\{product\.name\}\}', '', 'g') !~ '\{\{|\}\}'
    LIMIT 1
$$;
ALTER FUNCTION msgtemplates.sold_out_body(uuid, uuid, text, bigint) OWNER TO commerce_msgtemplates_writer;
REVOKE ALL ON FUNCTION msgtemplates.sold_out_body(uuid, uuid, text, bigint) FROM PUBLIC;
GRANT USAGE ON SCHEMA msgtemplates TO commerce_integration_writer;
GRANT EXECUTE ON FUNCTION msgtemplates.sold_out_body(uuid, uuid, text, bigint) TO commerce_integration_writer;
COMMENT ON FUNCTION msgtemplates.sold_out_body(uuid, uuid, text, bigint) IS
 'W3-04B (0151). Template body usable as the sold-out reply or NULL; only caller the integration-writer definers claims.set_sold_out_reply and integration.plan_claim_reply (explicit scope, no GUC).';

-- ---------------------------------------------------------------------------------------
-- A6 sold-out rule as a boolean-only definer (the 0116 formula; no raw balance leaves the function).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inventory.claim_sku_sold_out(p_sku uuid, p_quantity integer) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_tracked boolean; v_available bigint;
BEGIN
    IF p_sku IS NULL OR p_quantity IS NULL OR p_quantity NOT BETWEEN 1 AND 999
       OR current_setting('transaction_isolation') <> 'read committed'
       OR coalesce(current_setting('app.tenant_id', true), '') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
       OR coalesce(current_setting('app.store_id', true), '') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        RAISE EXCEPTION 'invalid sold-out request' USING ERRCODE = '22023';
    END IF;
    v_tenant := current_setting('app.tenant_id')::uuid;
    v_store := current_setting('app.store_id')::uuid;
    SELECT s.inventory_tracked INTO v_tracked FROM catalog.skus s WHERE s.tenant_id = v_tenant AND s.store_id = v_store AND s.id = p_sku;
    IF NOT FOUND OR NOT v_tracked THEN RETURN false; END IF; -- A6: an untracked SKU is never sold out
    SELECT coalesce(sum(b.on_hand - b.reserved - b.allocated - b.unavailable), 0)::bigint INTO v_available FROM inventory.balances b
     WHERE b.tenant_id = v_tenant AND b.store_id = v_store AND b.sku_id = p_sku;
    RETURN v_available < p_quantity;
END $$;
ALTER FUNCTION inventory.claim_sku_sold_out(uuid, integer) OWNER TO commerce_inventory_writer;
REVOKE ALL ON FUNCTION inventory.claim_sku_sold_out(uuid, integer) FROM PUBLIC;
GRANT USAGE ON SCHEMA inventory TO commerce_integration_writer;
GRANT EXECUTE ON FUNCTION inventory.claim_sku_sold_out(uuid, integer) TO commerce_integration_writer;
COMMENT ON FUNCTION inventory.claim_sku_sold_out(uuid, integer) IS
 'W3-04B (0151). A6 tracked AND available (sum on_hand-reserved-allocated-unavailable over the store''s warehouses) < quantity, scope from the app.tenant_id/app.store_id GUCs; boolean only, read-only, never a stock hold. EXECUTE commerce_integration_writer (claim reply planner) only.';

-- ---------------------------------------------------------------------------------------
-- Settings (per store). Absent row = enabled with the fixed template. FORCE RLS; no login role reads the table directly.
-- ---------------------------------------------------------------------------------------
CREATE TABLE claims.sold_out_settings (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    template_id text NOT NULL DEFAULT 'sold-out-reply/v1' CHECK (template_id ~ '^[a-z0-9][a-z0-9/_-]{0,63}$'),
    template_version bigint NOT NULL DEFAULT 1 CHECK (template_version > 0),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    principal_id uuid NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id)
);
ALTER TABLE claims.sold_out_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE claims.sold_out_settings FORCE ROW LEVEL SECURITY;
REVOKE ALL ON claims.sold_out_settings FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON claims.sold_out_settings TO commerce_integration_writer;
CREATE POLICY sold_out_settings_rw ON claims.sold_out_settings FOR ALL TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)) WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id));

-- Reads the planners need beyond 0064/0128 (columns only; scope policies of those migrations apply).
GRANT SELECT(offer_id, quantity) ON claims.events TO commerce_integration_writer;

-- Audit actions of this unit. The two planner actions are written inside the intake transaction (intake_scope), the setter's in the merchant one.
CREATE POLICY sold_out_reply_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s)
        AND action IN ('claim_reply_skipped:sold_out_off', 'claim_reply_sold_out'));
CREATE POLICY sold_out_settings_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id) AND action = 'claims.sold_out_reply.set');

-- ---------------------------------------------------------------------------------------
-- Private helper: the facts of the bundle-creating claim event. Zero rows when the event has no offer (not a claim). sold_out is the A6 rule
-- or an inactive offer; product_name is cut to 60 characters so the frozen text stays within the 2048-byte request bound.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.claim_sold_out_facts(p_tenant uuid, p_store uuid, p_inbox_event uuid)
RETURNS TABLE(sold_out boolean, reply_enabled boolean, tpl_id text, tpl_version bigint, offer uuid, product_name text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE e record; o record; st record; v_name text; v_sold boolean;
BEGIN
    SELECT x.offer_id, x.quantity INTO e FROM claims.events x
     WHERE x.tenant_id = p_tenant AND x.store_id = p_store AND x.source_event_id = p_inbox_event AND x.outcome = 'ACCEPTED' AND x.bundle_version = 1;
    IF NOT FOUND OR e.offer_id IS NULL OR e.quantity IS NULL THEN RETURN; END IF;
    SELECT f.active, f.sku_id INTO o FROM live.offers f WHERE f.tenant_id = p_tenant AND f.store_id = p_store AND f.id = e.offer_id;
    IF NOT FOUND THEN RETURN; END IF;
    -- inventory.claim_sku_sold_out: definer commerce_inventory_writer; A6 availability (0116 formula) for the claimed quantity.
    v_sold := NOT o.active OR inventory.claim_sku_sold_out(o.sku_id, e.quantity);
    SELECT s.enabled, s.template_id, s.template_version INTO st FROM claims.sold_out_settings s WHERE s.tenant_id = p_tenant AND s.store_id = p_store;
    IF NOT FOUND THEN st.enabled := true; st.template_id := 'sold-out-reply/v1'; st.template_version := 1; END IF;
    SELECT left(p.name, 60) INTO v_name FROM catalog.skus k
      JOIN catalog.products p ON p.tenant_id = k.tenant_id AND p.store_id = k.store_id AND p.id = k.product_id
     WHERE k.tenant_id = p_tenant AND k.store_id = p_store AND k.id = o.sku_id;
    RETURN QUERY SELECT v_sold, st.enabled, st.template_id, st.template_version, e.offer_id, coalesce(v_name, '');
END $$;
ALTER FUNCTION integration.claim_sold_out_facts(uuid, uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.claim_sold_out_facts(uuid, uuid, uuid) FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- claim_reply_plannable: sold-out + switch off = audited skip, patched in place from the live definition (the 0143 pattern: loud failure when
-- the definition is not the expected one; ownership re-asserted).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
    v_needle := ' ELSE RETURN ''OK'';';
    v_repl := ' ELSIF EXISTS(SELECT 1 FROM integration.claim_sold_out_facts(i.tenant_id,i.store_id,i.inbox_event_id) f WHERE f.sold_out AND NOT f.reply_enabled) THEN'
        || chr(10) || '  v_code:=''sold_out_off'';' || chr(10) || v_needle;
    v_fn := pg_get_functiondef('integration.claim_reply_plannable(uuid)'::regprocedure);
    IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
        RAISE EXCEPTION 'integration.claim_reply_plannable has an unexpected shape for the sold-out patch';
    END IF;
    EXECUTE replace(v_fn, v_needle, v_repl);
END $$;
ALTER FUNCTION integration.claim_reply_plannable(uuid) OWNER TO commerce_integration_writer;

-- ---------------------------------------------------------------------------------------
-- plan_claim_reply: the 0128 body plus the sold-out branch (signature, owner and grants unchanged).
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.plan_claim_reply(p_intake uuid,p_operation uuid,p_link_hash bytea,p_link_key_id text,p_job bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE i record; ev record; src record; b record; j record; v_domain uuid; v_origin text; v_expires timestamptz;
 v_deadline timestamptz; v_request jsonb; v_key text; dw record; v_known boolean:=false; v_gen bigint:=0;
 sf record; v_text text; v_tpl text; v_tplv bigint;
BEGIN
 IF p_intake IS NULL OR p_operation IS NULL OR p_link_hash IS NULL OR octet_length(p_link_hash)<>32
  OR p_link_key_id IS NULL OR p_link_key_id !~ '^[0-9a-f]{16}$' OR p_job IS NULL OR p_job<=0
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023';
 END IF;
 SELECT x.tenant_id,x.store_id,x.inbox_event_id,x.source_id,x.session_id,x.platform,x.object,x.asset_id,x.comment_ref,
  x.live_media,x.occurred_at,x.received_at,x.app_id INTO i FROM claims.meta_intake x
  WHERE x.id=p_intake AND x.lease_xid=pg_current_xact_id() AND x.state='PENDING';
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 -- Only the ACCEPTED event that CREATED the bundle plans a reply (bundle_version 1, IR-3).
 SELECT e.id,e.bundle_id INTO ev FROM claims.events e WHERE e.tenant_id=i.tenant_id AND e.store_id=i.store_id
  AND e.source_event_id=i.inbox_event_id AND e.outcome='ACCEPTED' AND e.bundle_version=1;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 SELECT y.id,y.binding_id,y.platform,y.asset_id,y.reply_locale,y.principal_id,y.active,y.private_reply INTO src
  FROM live.claim_sources y WHERE y.tenant_id=i.tenant_id AND y.store_id=i.store_id AND y.id=i.source_id;
 SELECT z.id,z.provider,z.external_asset_id,z.semantic_version,z.enabled INTO b FROM integration.bindings z
  WHERE z.tenant_id=i.tenant_id AND z.store_id=i.store_id AND z.id=src.binding_id;
 -- claim_reply_plannable held the locks since; any failure here is an invariant breach, not a normal path.
 IF src.id IS NULL OR NOT src.active OR NOT src.private_reply OR b.id IS NULL OR NOT b.enabled
  OR b.provider<>src.platform OR b.external_asset_id<>src.asset_id OR b.provider<>i.platform THEN
  RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023';
 END IF;
 SELECT d.id,d.origin INTO v_domain,v_origin FROM control.storefront_domains d JOIN control.storefront_publications p
  ON p.tenant_id=d.tenant_id AND p.store_id=d.store_id AND p.published
  WHERE d.tenant_id=i.tenant_id AND d.store_id=i.store_id AND d.state='ACTIVE' ORDER BY d.id LIMIT 1;
 IF v_domain IS NULL THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 -- The River job must be exactly the row inserted in this transaction (xmin needs table-level SELECT).
 SELECT r.id INTO j FROM river.river_job r WHERE r.id=p_job AND r.kind='external_operation_v1' AND r.queue='default'
  AND r.unique_key IS NULL AND r.args=jsonb_build_object('operation_id',p_operation::text,'version',1)
  AND r.xmin=pg_current_xact_id()::xid;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 -- §3.6 / P2-3: freeze the EFFECTIVE takeover generation of the actor's known conversation (via bundle peers), else 0.
 SELECT t.takeover_generation INTO dw FROM inbox.dm_window_for_bundle(i.tenant_id,i.store_id,ev.bundle_id,i.app_id,i.object,i.asset_id) t;
 IF FOUND THEN v_known:=true; v_gen:=dw.takeover_generation; END IF;
 -- W3-04B: is the claimed offer sold out (A6 rule)? claim_reply_plannable already returned OK, so a sold-out claim here has the switch ON.
 SELECT f.sold_out,f.reply_enabled,f.tpl_id,f.tpl_version,f.offer,f.product_name INTO sf
  FROM integration.claim_sold_out_facts(i.tenant_id,i.store_id,i.inbox_event_id) f;
 IF sf.sold_out IS TRUE THEN
  IF NOT sf.reply_enabled THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
  -- msgtemplates.sold_out_body: definer commerce_msgtemplates_writer; the store's chosen template, else the fixed default (never fail the claim).
  v_tpl:=sf.tpl_id; v_tplv:=sf.tpl_version;
  v_text:=msgtemplates.sold_out_body(i.tenant_id,i.store_id,v_tpl,v_tplv);
  IF v_text IS NULL THEN
   v_tpl:='sold-out-reply/v1'; v_tplv:=1;
   v_text:=msgtemplates.sold_out_body(i.tenant_id,i.store_id,v_tpl,v_tplv);
  END IF;
  IF v_text IS NULL THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
  v_text:=replace(v_text,'{{product.name}}',sf.product_name);
  -- No claims.links row: there is no link to prove at Check time (claims.check_meta_reply skips it for this message type).
  v_deadline:=least(i.occurred_at+interval '7 days'-interval '1 hour',
   CASE WHEN i.live_media THEN i.received_at+interval '15 minutes' END);
  v_request:=jsonb_build_object('v',1,'platform',i.platform,'source_id',i.source_id,'asset_id',i.asset_id,
   'comment_ref',i.comment_ref,'bundle_id',ev.bundle_id,'session_id',i.session_id,'offer_id',sf.offer,
   'locale',src.reply_locale,'template',v_tpl,'template_version',v_tplv,'policy','mpr-policy/v1',
   'message_type','sold_out_reply','origin_kind','auto','text',v_text,'conversation_known',v_known,'takeover_generation',v_gen,
   'app_id',i.app_id,
   'deadline_at',to_char(v_deadline AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'live_media',i.live_media);
 ELSE
  v_expires:=claims.issue_system_link(p_intake,p_link_hash);
  v_deadline:=least(i.occurred_at+interval '7 days'-interval '1 hour',v_expires-interval '10 minutes',
   CASE WHEN i.live_media THEN i.received_at+interval '15 minutes' END);
  v_request:=jsonb_build_object('v',1,'platform',i.platform,'source_id',i.source_id,'asset_id',i.asset_id,
   'comment_ref',i.comment_ref,'bundle_id',ev.bundle_id,'session_id',i.session_id,'link_generation',1,
   'link_key_id',p_link_key_id,'locale',src.reply_locale,'template','claim-link/v1','policy','mpr-policy/v1',
   'message_type','first_private_reply','origin_kind','auto','conversation_known',v_known,'takeover_generation',v_gen,
   'app_id',i.app_id,'origin_ref',v_domain,'origin',v_origin,
   'deadline_at',to_char(v_deadline AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'live_media',i.live_media);
 END IF;
 IF octet_length(v_request::text)>2048 THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 v_key:='mpr:'||substr(encode(sha256(convert_to(i.object||'|'||i.asset_id||'|'||i.comment_ref,'UTF8')),'hex'),1,48);
 INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,external_asset_id,
  purpose,action,semantic_key,request_hash,request,job_id)
 VALUES(i.tenant_id,i.store_id,p_operation,src.principal_id,b.id,b.semantic_version,b.provider,b.external_asset_id,
  'service','meta.private_reply',v_key,sha256(convert_to(v_request::text,'UTF8')),v_request,p_job);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(i.tenant_id,i.store_id,p_operation,0,'READY','','operation_planned');
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
  VALUES(i.tenant_id,i.store_id,src.principal_id,'meta.private_reply.planned');
 IF sf.sold_out IS TRUE THEN
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(i.tenant_id,i.store_id,src.principal_id,'claim_reply_sold_out');
 END IF;
 RETURN p_operation;
END $$;
ALTER FUNCTION integration.plan_claim_reply(uuid,uuid,bytea,text,bigint) OWNER TO commerce_integration_writer;

-- ---------------------------------------------------------------------------------------
-- claims.check_meta_reply: the 0128 body; a sold_out_reply has no claim link, so the link proof is skipped for that message type only.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION claims.check_meta_reply(p_operation uuid,p_hash bytea) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o record; src record; v_deadline timestamptz; v_live boolean; v_ok boolean; dw record;
BEGIN
 IF p_operation IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32
  OR nullif(current_setting('app.tenant_id',true),'') IS NOT NULL OR nullif(current_setting('app.store_id',true),'') IS NOT NULL
  OR nullif(current_setting('app.principal_id',true),'') IS NOT NULL OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
  RAISE EXCEPTION 'invalid meta reply check' USING ERRCODE='22023';
 END IF;
 SELECT x.tenant_id,x.store_id,x.request INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.private_reply';
 IF NOT FOUND OR jsonb_typeof(o.request)<>'object' THEN
  RAISE EXCEPTION 'invalid meta reply check' USING ERRCODE='22023';
 END IF;
 v_deadline:=(o.request->>'deadline_at')::timestamptz;
 v_live:=(o.request->>'live_media')::boolean;
 IF clock_timestamp()>=v_deadline THEN RETURN 'deadline'; END IF;
 SELECT s.active,s.private_reply,s.principal_id,s.session_id INTO src FROM live.claim_sources s
  WHERE s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=(o.request->>'source_id')::uuid;
 IF NOT FOUND OR NOT src.active OR NOT src.private_reply THEN RETURN 'source_off'; END IF;
 IF NOT identity.principal_holds(o.tenant_id,o.store_id,src.principal_id,ARRAY['live:manage','integration:execute']) THEN
  RETURN 'principal_revoked';
 END IF;
 IF o.request->>'message_type' IS DISTINCT FROM 'sold_out_reply' THEN
  SELECT true INTO v_ok FROM claims.links k WHERE k.tenant_id=o.tenant_id AND k.store_id=o.store_id
   AND k.bundle_id=(o.request->>'bundle_id')::uuid AND k.generation=1 AND k.token_hash=p_hash
   AND k.expires_at>clock_timestamp()+interval '10 minutes';
  IF v_ok IS NOT TRUE THEN RETURN 'link_invalid'; END IF;
 END IF;
 IF v_live AND NOT EXISTS(SELECT 1 FROM live.claim_windows w WHERE w.tenant_id=o.tenant_id AND w.store_id=o.store_id
  AND w.session_id=src.session_id AND w.state='OPEN') THEN
  RETURN 'live_closed';
 END IF;
 -- §3.6: resolve the actor's conversation through the bundle's peers (conversation_known=false) or the frozen generation.
 IF o.request->>'origin_kind'='auto' AND o.request->>'app_id' ~ '^[0-9]{1,40}$' THEN
  SELECT t.mode,t.takeover_generation INTO dw FROM inbox.dm_window_for_bundle(o.tenant_id,o.store_id,(o.request->>'bundle_id')::uuid,
   o.request->>'app_id',CASE o.request->>'platform' WHEN 'facebook' THEN 'page' ELSE 'instagram' END,o.request->>'asset_id') t;
  IF FOUND THEN
   IF dw.mode='human' THEN RETURN 'human_takeover'; END IF;
   IF (o.request->>'conversation_known')::boolean AND dw.takeover_generation<>(o.request->>'takeover_generation')::bigint THEN
    RETURN 'takeover_changed';
   END IF;
  END IF;
 END IF;
 RETURN 'OK';
END $$;
ALTER FUNCTION claims.check_meta_reply(uuid,bytea) OWNER TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- live.console_marks: show a sold_out_reply as kind out_of_stock (patched in place; the line is unique in the 0123 definition).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
    v_needle := 'WHEN v_reply.message_type=''out_of_stock_reply'' THEN ''out_of_stock''';
    v_repl := 'WHEN v_reply.message_type IN (''out_of_stock_reply'',''sold_out_reply'') THEN ''out_of_stock''';
    v_fn := pg_get_functiondef('live.console_marks(uuid,text[])'::regprocedure);
    IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
        RAISE EXCEPTION 'live.console_marks has an unexpected shape for the sold-out patch';
    END IF;
    EXECUTE replace(v_fn, v_needle, v_repl);
END $$;
ALTER FUNCTION live.console_marks(uuid,text[]) OWNER TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- Merchant definers (owner commerce_integration_writer, EXECUTE commerce_runtime; scope and principal from the pinned GUCs).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.get_sold_out_reply()
RETURNS TABLE(enabled boolean, template_id text, template_version bigint, version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    RETURN QUERY SELECT s.enabled, s.template_id, s.template_version, s.version FROM claims.sold_out_settings s
     WHERE s.tenant_id = v[1] AND s.store_id = v[2];
    IF NOT FOUND THEN
        RETURN QUERY SELECT true, 'sold-out-reply/v1'::text, 1::bigint, 0::bigint; -- version 0 = never saved (the CAS base of the first save)
    END IF;
END $$;

CREATE FUNCTION claims.set_sold_out_reply(p_enabled boolean, p_template_id text, p_template_version bigint, p_expected_version bigint)
RETURNS TABLE(enabled boolean, template_id text, template_version bigint, version bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_rows bigint;
BEGIN
    IF p_enabled IS NULL OR p_expected_version IS NULL OR p_expected_version < 0 OR p_template_version IS NULL OR p_template_version < 1
       OR p_template_id IS NULL OR p_template_id !~ '^[a-z0-9][a-z0-9/_-]{0,63}$' THEN
        RAISE EXCEPTION 'invalid sold-out settings' USING ERRCODE = '22023';
    END IF;
    v := inbox.lcn_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:manage']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    -- msgtemplates.sold_out_body: definer commerce_msgtemplates_writer; the template must be a usable sold-out text of THIS store (or the fixed one).
    IF msgtemplates.sold_out_body(v[1], v[2], p_template_id, p_template_version) IS NULL THEN
        RAISE EXCEPTION 'invalid sold-out template' USING ERRCODE = '22023';
    END IF;
    IF p_expected_version = 0 THEN
        INSERT INTO claims.sold_out_settings AS x(tenant_id, store_id, enabled, template_id, template_version, version, principal_id)
         VALUES (v[1], v[2], p_enabled, p_template_id, p_template_version, 1, v[3]) ON CONFLICT (tenant_id, store_id) DO NOTHING;
    ELSE
        UPDATE claims.sold_out_settings x SET enabled = p_enabled, template_id = p_template_id, template_version = p_template_version,
               version = x.version + 1, principal_id = v[3], updated_at = clock_timestamp()
         WHERE x.tenant_id = v[1] AND x.store_id = v[2] AND x.version = p_expected_version;
    END IF;
    GET DIAGNOSTICS v_rows = ROW_COUNT;
    IF v_rows <> 1 THEN RAISE EXCEPTION 'version conflict' USING ERRCODE = 'PT409'; END IF;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v[1], v[2], v[3], 'claims.sold_out_reply.set');
    RETURN QUERY SELECT s.enabled, s.template_id, s.template_version, s.version FROM claims.sold_out_settings s
     WHERE s.tenant_id = v[1] AND s.store_id = v[2];
END $$;
DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY['claims.get_sold_out_reply()', 'claims.set_sold_out_reply(boolean,text,bigint,bigint)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_integration_writer', f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
        EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime', f);
    END LOOP;
END $$;
COMMENT ON FUNCTION claims.get_sold_out_reply() IS
 'W3-04B (0151). internal/claims GetSoldOutReply (commerce_runtime merchant transaction, live:read): the store''s sold-out reply switch and template; defaults (enabled, sold-out-reply/v1) at version 0 when never saved.';
COMMENT ON FUNCTION claims.set_sold_out_reply(boolean,text,bigint,bigint) IS
 'W3-04B (0151). internal/claims SetSoldOutReply (commerce_runtime merchant transaction, live:manage): compare-and-swap on version (0 = first save), template must pass msgtemplates.sold_out_body, audit claims.sold_out_reply.set.';
