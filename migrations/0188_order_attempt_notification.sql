-- Purpose: private transaction-local initial payment notification planning.
-- Depends on: 0171 provenance,0178 protected fence,0184 native Begin,0187 pay entry,
-- 0023 origin eligibility and 0128 native encrypted send artifacts.
-- Used by: the C2 coordinator after all native order/pay-entry work in its outer Tx.
-- Invariants: VOID before C3 has NULL operation/no job; READY has an exact native
-- pair/job; no browser reauthentication, fabricated binding, raw proof/credential
-- grants, provider IO or commit. OA2 owns C3/cutover and the dormant URL consumer.

CREATE TABLE inbox.order_payment_notifications (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 owner_id uuid NOT NULL, order_id uuid NOT NULL, create_attempt_id uuid,
 kind text NOT NULL CHECK(kind IN ('initial','reminder')),
 semantic_key bytea NOT NULL CHECK(octet_length(semantic_key)=32),
 link_version bigint NOT NULL CHECK(link_version>0), conversation_id uuid, binding_id uuid,
 operation_id uuid UNIQUE, outbound_id uuid,
 state text NOT NULL CHECK(state IN ('VOID','READY','DISPATCHING','ACCEPTED','DELIVERY_UNKNOWN')),
 void_stage text NOT NULL CHECK(void_stage IN ('none','before_c3','after_c3')),
 void_code text CHECK(void_code IN ('forbidden','window_closed','capability','conversation_gone',
  'takeover_changed','duplicate_recent','rate_limited','template_not_found','template_kind',
  'recipient_invalid','format_invalid','pay_entry_unavailable','storefront_unavailable','domain_selection_required')),
 dispatch_authorized_at timestamptz CHECK(isfinite(dispatch_authorized_at)),
 worker_generation bigint NOT NULL DEFAULT 0 CHECK(worker_generation>=0),
 UNIQUE(tenant_id,store_id,semantic_key),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id,create_attempt_id)
  REFERENCES checkout.orders(tenant_id,store_id,owner_id,id,create_attempt_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,create_attempt_id)
  REFERENCES ops.order_attempt_creators(tenant_id,store_id,buyer_owner_id,attempt_id),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,operation_id) REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,outbound_id) REFERENCES inbox.outbound_messages(tenant_id,store_id,id),
 CHECK(kind<>'initial' OR (create_attempt_id IS NOT NULL AND link_version=1
  AND semantic_key=sha256(convert_to('initial|'||create_attempt_id::text,'UTF8')))),
 CHECK(operation_id IS NOT NULL OR (state='VOID' AND void_stage='before_c3' AND dispatch_authorized_at IS NULL)),
 CHECK((operation_id IS NULL)=(outbound_id IS NULL)),
 CHECK(binding_id IS NOT NULL OR (state='VOID' AND operation_id IS NULL AND void_stage='before_c3')),
 CHECK((state='VOID' AND void_stage IN ('before_c3','after_c3') AND void_code IS NOT NULL)
  OR (state<>'VOID' AND void_stage='none' AND void_code IS NULL)),
 CHECK(state NOT IN ('DISPATCHING','ACCEPTED','DELIVERY_UNKNOWN') OR dispatch_authorized_at IS NOT NULL)
);
ALTER TABLE inbox.order_payment_notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.order_payment_notifications FORCE ROW LEVEL SECURITY;
REVOKE ALL ON inbox.order_payment_notifications FROM PUBLIC;
GRANT SELECT,INSERT ON inbox.order_payment_notifications TO commerce_integration_writer;
CREATE POLICY order_payment_notification_read ON inbox.order_payment_notifications FOR SELECT TO commerce_integration_writer
 USING(inbox.lcn_in_scope(tenant_id,store_id));
CREATE POLICY order_payment_notification_create ON inbox.order_payment_notifications FOR INSERT TO commerce_integration_writer
 WITH CHECK(inbox.lcn_in_scope(tenant_id,store_id) AND kind='initial' AND create_attempt_id=order_id
  AND link_version=1 AND worker_generation=0 AND dispatch_authorized_at IS NULL AND state IN ('VOID','READY'));

CREATE FUNCTION inbox.guard_order_payment_notification() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF (NEW.id,NEW.tenant_id,NEW.store_id,NEW.owner_id,NEW.order_id,NEW.create_attempt_id,
  NEW.kind,NEW.semantic_key,NEW.link_version,NEW.conversation_id)
  IS DISTINCT FROM (OLD.id,OLD.tenant_id,OLD.store_id,OLD.owner_id,OLD.order_id,OLD.create_attempt_id,
  OLD.kind,OLD.semantic_key,OLD.link_version,OLD.conversation_id)
  OR (OLD.operation_id IS NOT NULL AND (NEW.operation_id,NEW.outbound_id,NEW.binding_id)
   IS DISTINCT FROM (OLD.operation_id,OLD.outbound_id,OLD.binding_id))
  OR (OLD.dispatch_authorized_at IS NOT NULL AND NEW.dispatch_authorized_at IS DISTINCT FROM OLD.dispatch_authorized_at)
  OR (OLD.state='VOID' AND NEW.state IS DISTINCT FROM 'VOID') THEN
  RAISE EXCEPTION 'notification_identity_immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inbox.guard_order_payment_notification() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.guard_order_payment_notification() FROM PUBLIC;
CREATE TRIGGER order_payment_notification_identity BEFORE UPDATE ON inbox.order_payment_notifications
 FOR EACH ROW EXECUTE FUNCTION inbox.guard_order_payment_notification();

GRANT USAGE ON SCHEMA ops,buyer,checkout,social,msgtemplates TO commerce_integration_writer;
GRANT USAGE ON SCHEMA inbox TO commerce_order_attempt_owner;
GRANT EXECUTE ON FUNCTION ops.order_attempt_execution_context(uuid,text),social.read_thread(uuid,bigint,integer),
 msgtemplates.resolve(text,bigint) TO commerce_integration_writer;

CREATE FUNCTION checkout.order_attempt_notification_payable(p_attempt uuid,p_plan text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; b jsonb; tenant uuid; store uuid; owner_id uuid; order_key bigint; result boolean;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan); b:=ctx->'binding';
 tenant:=(b->>'tenant_id')::uuid; store:=(b->>'store_id')::uuid; owner_id:=(b->>'buyer_owner_id')::uuid;
 order_key:=hashtextextended('checkout.order|'||tenant::text||'|'||store::text||'|'||p_attempt::text,0);
 IF NOT EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=pg_backend_pid() AND l.locktype='advisory'
  AND l.classid=((order_key>>32)&4294967295)::oid AND l.objid=(order_key&4294967295)::oid
  AND l.objsubid=1 AND l.mode='ExclusiveLock' AND l.granted) THEN
  RAISE EXCEPTION 'order_not_locked' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',b->>'principal_id',true),set_config('app.buyer_id',owner_id::text,true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 SELECT EXISTS(SELECT 1 FROM checkout.orders o JOIN checkout.order_pay_entries e
  ON (e.tenant_id,e.store_id,e.owner_id,e.order_id,e.create_attempt_id)=(o.tenant_id,o.store_id,o.owner_id,o.id,o.create_attempt_id)
  JOIN inventory.reservations r ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id AND r.id=o.id
  WHERE o.tenant_id=tenant AND o.store_id=store AND o.owner_id=order_attempt_notification_payable.owner_id
   AND o.id=p_attempt AND o.create_attempt_id=p_attempt AND o.creator_kind='order_attempt'
   AND o.creator_session_id IS NULL AND o.source='merchant_manual' AND o.generation=1 AND o.expires_at>clock_timestamp()
   AND e.link_version=1 AND e.revoked_at IS NULL AND e.issued_at<=clock_timestamp()
   AND e.expires_at>clock_timestamp() AND e.expires_at<=o.expires_at
   AND EXISTS(SELECT 1 FROM control.tenants t WHERE t.id=tenant AND t.active)
   AND EXISTS(SELECT 1 FROM control.stores s WHERE s.tenant_id=tenant AND s.id=store AND s.active)
   AND EXISTS(SELECT 1 FROM buyer.owners w WHERE w.tenant_id=tenant AND w.store_id=store AND w.id=o.owner_id AND w.active)
   AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts p WHERE p.tenant_id=tenant AND p.store_id=store AND p.order_id=o.id)
   AND r.checkout_id=o.id AND r.buyer_owner_id=o.owner_id AND r.buyer_session_id IS NULL
   AND r.creator_kind='order_attempt' AND r.create_attempt_id=p_attempt AND r.generation=o.generation
   AND ((o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER' AND r.state='HELD'
    AND r.expires_at>clock_timestamp() AND EXISTS(SELECT 1 FROM checkout.bank_transfers x
     WHERE x.tenant_id=tenant AND x.store_id=store AND x.owner_id=o.owner_id AND x.order_id=o.id AND x.state IN ('AWAITING','SUBMITTED','REJECTED')))
    OR (o.payment_mode='pay_at_pickup' AND o.commercial_state='CONFIRMED' AND o.collection_state='PENDING' AND r.state='COMMITTED')
    OR (o.payment_mode='cash_on_delivery' AND o.commercial_state='AWAITING_COLLECTION' AND o.collection_state='PENDING' AND r.state='COMMITTED')))
 INTO result;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION checkout.order_attempt_notification_payable(uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_notification_payable(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_notification_payable(uuid,text) TO commerce_integration_writer;

CREATE FUNCTION buyer.order_attempt_storefront_origin(p_attempt uuid,p_plan text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; tenant uuid; store uuid; candidate record; candidates integer:=0; t0 timestamptz; t1 timestamptz;
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan);
 tenant:=(ctx->'binding'->>'tenant_id')::uuid; store:=(ctx->'binding'->>'store_id')::uuid;
 t0:=clock_timestamp();
 -- Same eligibility-before-LIMIT2 kernel as0023; no replacement after sole expiry.
 FOR candidate IN SELECT d.origin,d.ownership_verified_at,d.tls_verified_at,d.valid_until
  FROM control.storefront_domains d JOIN control.storefront_publications p USING(tenant_id,store_id)
  JOIN control.stores s ON s.tenant_id=d.tenant_id AND s.id=d.store_id JOIN control.tenants t ON t.id=d.tenant_id
  WHERE d.tenant_id=tenant AND d.store_id=store AND s.active AND t.active AND p.published AND d.state='ACTIVE'
   AND d.ownership_verified_at<=t0 AND d.tls_verified_at<=t0 AND d.valid_until>t0
  ORDER BY d.origin COLLATE "C" LIMIT 2 LOOP
  candidates:=candidates+1;
  IF candidates=2 THEN RETURN jsonb_build_object('state','domain_selection_required','origin',NULL); END IF;
 END LOOP;
 IF candidates=0 THEN RETURN jsonb_build_object('state','storefront_unavailable','origin',NULL); END IF;
 t1:=clock_timestamp();
 IF candidate.ownership_verified_at>t1 OR candidate.tls_verified_at>t1 OR candidate.valid_until<=t1 THEN
  RETURN jsonb_build_object('state','storefront_unavailable','origin',NULL); END IF;
 RETURN jsonb_build_object('state','configured','origin',candidate.origin);
END $$;
ALTER FUNCTION buyer.order_attempt_storefront_origin(uuid,text) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.order_attempt_storefront_origin(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.order_attempt_storefront_origin(uuid,text) TO commerce_integration_writer;

CREATE FUNCTION inbox.order_attempt_prepare_notification(p_attempt uuid,p_plan text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; b jsonb; selection jsonb; tenant uuid; store uuid; principal uuid; conv uuid;
 c record; st record; binding record; tpl record; thread record; origin jsonb;
 result jsonb; code text; now_at timestamptz; generation bigint;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true);
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan); b:=ctx->'binding';
 selection:=ctx->'plan'->'facts'->'selection';
 tenant:=(b->>'tenant_id')::uuid; store:=(b->>'store_id')::uuid; principal:=(b->>'principal_id')::uuid;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',principal::text,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true);
 IF NOT checkout.order_attempt_notification_payable(p_attempt,p_plan) THEN code:='pay_entry_unavailable';
 ELSIF selection->>'send_payment_link' IS DISTINCT FROM 'true' THEN
  result:=jsonb_build_object('state','NOT_REQUESTED');
 ELSE
  conv:=(selection->'for'->>'conversation_id')::uuid;
  IF NOT identity.principal_holds(tenant,store,principal,ARRAY['inbox:reply','inbox:read']) THEN code:='forbidden';
  ELSE
   SELECT x.id,x.object,x.asset_id,x.app_id,x.peer_key INTO c FROM social.conversations x
    WHERE x.id=conv AND x.tenant_id=tenant AND x.store_id=store;
   IF NOT FOUND THEN code:='conversation_gone';
   ELSE
    SELECT s.mode,s.assignee_principal,s.takeover_generation,s.human_until,s.last_inbound_at INTO st
     FROM inbox.conversation_state s WHERE s.tenant_id=tenant AND s.store_id=store AND s.conversation_id=conv FOR UPDATE;
    now_at:=clock_timestamp();
    IF NOT FOUND THEN code:='conversation_gone';
    ELSIF st.last_inbound_at IS NULL OR st.last_inbound_at+interval '24 hours'-interval '5 minutes'<=now_at THEN code:='window_closed';
    ELSE
     SELECT z.id,z.semantic_version INTO binding FROM integration.bindings z
      WHERE z.tenant_id=tenant AND z.store_id=store AND z.provider=CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END
       AND z.external_asset_id=c.asset_id AND z.enabled ORDER BY z.id LIMIT 1 FOR SHARE;
     IF NOT FOUND OR coalesce(integration.binding_capability_state(tenant,store,binding.id,'dm_session',ARRAY[]::text[]),'unknown')
      NOT IN ('ok','review_required') THEN code:='capability';
     ELSE
      origin:=buyer.order_attempt_storefront_origin(p_attempt,p_plan);
      IF origin->>'state'<>'configured' THEN code:=origin->>'state';
      ELSE
       SELECT x.* INTO tpl FROM msgtemplates.resolve('order-pay-link/v1',1) x;
       IF NOT FOUND THEN code:='template_not_found';
       ELSIF NOT tpl.fixed OR NOT ('dm'=ANY(tpl.kinds)) OR
        (length(tpl.body)-length(replace(tpl.body,'{{連結}}','')))/length('{{連結}}')<>1 THEN code:='template_kind';
       ELSIF EXISTS(SELECT 1 FROM integration.operations o WHERE o.tenant_id=tenant AND o.store_id=store
        AND o.action='meta.dm_send' AND o.request->>'conversation_id'=conv::text
        AND o.request->>'order_id'=p_attempt::text AND o.request->>'template_id'='order-pay-link/v1'
        AND o.request->>'template_version'='1' AND o.created_at>clock_timestamp()-interval '30 seconds') THEN
        code:='duplicate_recent';
       ELSE
        BEGIN
         PERFORM inbox.lcn_rate_check(tenant,store);
        EXCEPTION WHEN SQLSTATE 'PT429' THEN code:='rate_limited';
        END;
        IF code IS NULL THEN
         -- This is an encrypted internal DTO, not a merchant/body/recipient view.
         SELECT x.* INTO thread FROM social.read_thread(conv,NULL::bigint,1) x;
         IF NOT FOUND THEN code:='conversation_gone';
         ELSE
          generation:=st.takeover_generation;
          IF st.mode='human' AND st.human_until IS NOT NULL AND now_at>=st.human_until THEN generation:=generation+1; END IF;
          result:=jsonb_build_object('state','PREPARED','tenant_id',tenant,'store_id',store,'order_id',p_attempt,
           'link_version',1,'conversation_id',conv,'binding_id',binding.id,'takeover_generation',generation,
           'platform',CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END,
           'template_id',tpl.template_id,'template_version',tpl.version,'template_body',tpl.body,
           'origin',origin->>'origin','locale',selection->>'locale',
           'thread_row',jsonb_build_object('event_id',thread.event_id,'key_id',thread.key_id,
            'nonce_hex',encode(thread.nonce,'hex'),'ciphertext_hex',encode(thread.ciphertext,'hex'),
            'app_id',thread.app_id,'object',thread.object,'asset_id',thread.asset_id,'event_key',thread.event_key,
            'payload_hash',thread.payload_hash,'tenant_id',thread.tenant_id,'store_id',thread.store_id,
            'route_id',thread.route_id,'route_epoch',thread.route_epoch,'server_seq',thread.server_seq,
            'occurred_at',thread.occurred_at,'direction',thread.direction));
         END IF;
        END IF;
       END IF;
      END IF;
     END IF;
    END IF;
   END IF;
  END IF;
 END IF;
 IF code IS NOT NULL THEN result:=jsonb_build_object('state','VOID','void_code',code,'conversation_id',conv,'link_version',1); END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RAISE;
END $$;
ALTER FUNCTION inbox.order_attempt_prepare_notification(uuid,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.order_attempt_prepare_notification(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.order_attempt_prepare_notification(uuid,text) TO commerce_order_attempt_owner;

CREATE FUNCTION inbox.order_attempt_finish_notification(p_attempt uuid,p_plan text,p_material jsonb) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; b jsonb; prepared jsonb; tenant uuid; store uuid; principal uuid; buyer_id uuid;
 n inbox.order_payment_notifications%ROWTYPE; c record; st record; binding record;
 operation uuid; outbound uuid; job bigint; notification uuid; generation bigint; provider text;
 digest bytea; nonce bytea; ciphertext bytea; enc bytea; sealed bytea; request jsonb; result jsonb; code text;
 now_at timestamptz; semantic bytea;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true);
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan); b:=ctx->'binding';
 tenant:=(b->>'tenant_id')::uuid; store:=(b->>'store_id')::uuid; principal:=(b->>'principal_id')::uuid;
 buyer_id:=(b->>'buyer_owner_id')::uuid; semantic:=sha256(convert_to('initial|'||p_attempt::text,'UTF8'));
 IF jsonb_typeof(p_material) IS DISTINCT FROM 'object' OR octet_length(p_material::text)>65536 THEN
  RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
 IF p_material->>'state'='NOT_REQUESTED' THEN
  IF p_material IS DISTINCT FROM jsonb_build_object('state','NOT_REQUESTED') THEN
   RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
 ELSIF p_material->>'state'='VOID' THEN
  IF p_material IS DISTINCT FROM jsonb_build_object('state','VOID','void_code',p_material->>'void_code')
   OR p_material->>'void_code' IS NULL THEN RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
 ELSIF p_material->>'state'='READY' THEN
  IF (p_material-ARRAY['state','operation_id','outbound_id','job_id','body_hmac_hex','key_id','nonce_hex','ciphertext_hex',
    'dispatch_enc_hex','dispatch_ciphertext_hex','template_id','template_version'])<>'{}'::jsonb
   OR (SELECT count(*) FROM jsonb_object_keys(p_material))<>12
   OR EXISTS(SELECT 1 FROM jsonb_each(p_material) e WHERE jsonb_typeof(e.value) IS DISTINCT FROM
    CASE WHEN e.key IN ('job_id','template_version') THEN 'number' ELSE 'string' END)
   OR p_material->>'template_id' IS DISTINCT FROM 'order-pay-link/v1' OR p_material->>'template_version' IS DISTINCT FROM '1'
   OR p_material->>'key_id' !~ '^[A-Za-z0-9_-]{1,64}$'
   OR p_material->>'body_hmac_hex' !~ '^[0-9a-f]{64}$' OR p_material->>'nonce_hex' !~ '^[0-9a-f]{24}$'
   OR p_material->>'ciphertext_hex' !~ '^[0-9a-f]+$' OR p_material->>'dispatch_enc_hex' !~ '^[0-9a-f]{64}$'
   OR p_material->>'dispatch_ciphertext_hex' !~ '^[0-9a-f]+$' THEN
   RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
  BEGIN
   operation:=(p_material->>'operation_id')::uuid; outbound:=(p_material->>'outbound_id')::uuid; job:=(p_material->>'job_id')::bigint;
   digest:=decode(p_material->>'body_hmac_hex','hex'); nonce:=decode(p_material->>'nonce_hex','hex');
   ciphertext:=decode(p_material->>'ciphertext_hex','hex'); enc:=decode(p_material->>'dispatch_enc_hex','hex');
   sealed:=decode(p_material->>'dispatch_ciphertext_hex','hex');
  EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range THEN
   RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400';
  END;
  IF operation=outbound OR job<=0 OR octet_length(ciphertext) NOT BETWEEN 17 AND 16384
   OR octet_length(sealed) NOT BETWEEN 17 AND 16384 THEN
   RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
 ELSE RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400';
 END IF;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',principal::text,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true);
 SELECT x.* INTO n FROM inbox.order_payment_notifications x WHERE x.tenant_id=tenant AND x.store_id=store AND x.semantic_key=semantic;
 IF FOUND THEN
  IF n.state IS DISTINCT FROM p_material->>'state' OR n.owner_id IS DISTINCT FROM buyer_id
   OR n.create_attempt_id IS DISTINCT FROM p_attempt OR n.order_id IS DISTINCT FROM p_attempt
   OR (n.state='VOID' AND n.void_code IS DISTINCT FROM p_material->>'void_code')
   OR (n.state='READY' AND (n.operation_id IS DISTINCT FROM operation OR n.outbound_id IS DISTINCT FROM outbound
    OR NOT EXISTS(SELECT 1 FROM integration.operations o WHERE o.id=operation AND o.tenant_id=tenant AND o.store_id=store AND o.job_id=job))) THEN
   RAISE EXCEPTION 'notification_material_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  prepared:=inbox.order_attempt_prepare_notification(p_attempt,p_plan);
  IF prepared->>'state'='NOT_REQUESTED' THEN
   IF p_material->>'state' IS DISTINCT FROM 'NOT_REQUESTED' THEN RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
   result:=prepared;
  ELSIF prepared->>'state'='VOID' THEN
   code:=prepared->>'void_code';
   IF p_material->>'state'='READY' THEN
    RAISE EXCEPTION 'notification_reprepare' USING ERRCODE='PT409'; END IF;
   IF p_material->>'state' IS DISTINCT FROM 'VOID' OR p_material->>'void_code' IS DISTINCT FROM code THEN
    RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
   INSERT INTO inbox.order_payment_notifications(tenant_id,store_id,owner_id,order_id,create_attempt_id,kind,semantic_key,
    link_version,conversation_id,state,void_stage,void_code)
   VALUES(tenant,store,buyer_id,p_attempt,p_attempt,'initial',semantic,1,(prepared->>'conversation_id')::uuid,'VOID','before_c3',code)
   RETURNING * INTO n;
  ELSE
   IF p_material->>'state' IS DISTINCT FROM 'READY' OR prepared->>'state' IS DISTINCT FROM 'PREPARED' THEN
    RAISE EXCEPTION 'notification_material_invalid' USING ERRCODE='PT400'; END IF;
   SELECT x.id,x.object,x.asset_id,x.app_id,x.peer_key INTO c FROM social.conversations x
    WHERE x.id=(prepared->>'conversation_id')::uuid AND x.tenant_id=tenant AND x.store_id=store;
   SELECT s.mode,s.assignee_principal,s.takeover_generation,s.human_until,s.last_inbound_at INTO st
    FROM inbox.conversation_state s WHERE s.tenant_id=tenant AND s.store_id=store AND s.conversation_id=c.id;
   SELECT z.id,z.semantic_version INTO binding FROM integration.bindings z
    WHERE z.id=(prepared->>'binding_id')::uuid AND z.tenant_id=tenant AND z.store_id=store;
   now_at:=clock_timestamp(); generation:=(prepared->>'takeover_generation')::bigint;
   IF EXISTS(SELECT 1 FROM integration.operations o WHERE o.tenant_id=tenant AND o.store_id=store AND o.action='meta.dm_send'
    AND o.request->>'conversation_id'=c.id::text AND o.request->>'body_hmac'=encode(digest,'hex')
    AND o.created_at>now_at-interval '30 seconds') THEN RAISE EXCEPTION 'notification_reprepare' USING ERRCODE='PT409'; END IF;
   IF st.mode<>'human' OR st.assignee_principal IS DISTINCT FROM principal OR (st.human_until IS NOT NULL AND now_at>=st.human_until) THEN
    generation:=generation+1;
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(tenant,store,principal,'inbox.takeover');
   END IF;
   UPDATE inbox.conversation_state SET mode='human',assignee_principal=principal,takeover_generation=generation,
    last_human_outbound_at=now_at,human_until=now_at+interval '6 hours',version=version+1,updated_at=now_at
    WHERE tenant_id=tenant AND store_id=store AND conversation_id=c.id;
   notification:=gen_random_uuid(); provider:=prepared->>'platform';
   request:=jsonb_build_object('v',1,'kind','dm','message_type','dm','origin','human','platform',provider,
    'asset_id',c.asset_id,'app_id',c.app_id,'conversation_id',c.id,'conversation_known',true,'peer_key',c.peer_key,
    'outbound_id',outbound,'body_hmac',encode(digest,'hex'),'policy','order-attempt-notification/v1',
    'notification_id',notification,'takeover_generation',generation,'principal_id',principal,'order_id',p_attempt,'link_version',1,
    'template_id','order-pay-link/v1','template_version',1,
    'deadline_at',to_char((st.last_inbound_at+interval '24 hours'-interval '5 minutes') AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
   PERFORM inbox.lcn_emit(tenant,store,principal,'dm','meta.dm_send',binding.id,binding.semantic_version,provider,c.asset_id,
    'oan:'||encode(semantic,'hex'),request,operation,job,outbound,c.id,NULL,digest,p_material->>'key_id',nonce,ciphertext,enc,sealed,
    'order-pay-link/v1',1,'inbox.dm.planned');
   INSERT INTO inbox.order_payment_notifications(id,tenant_id,store_id,owner_id,order_id,create_attempt_id,kind,semantic_key,
    link_version,conversation_id,binding_id,operation_id,outbound_id,state,void_stage)
   VALUES(notification,tenant,store,buyer_id,p_attempt,p_attempt,'initial',semantic,1,c.id,binding.id,operation,outbound,'READY','none')
   RETURNING * INTO n;
  END IF;
 END IF;
 IF result IS NULL THEN result:=jsonb_build_object('notification_id',n.id,'state',n.state,'operation_id',n.operation_id,'void_code',n.void_code); END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RAISE;
END $$;
ALTER FUNCTION inbox.order_attempt_finish_notification(uuid,text,jsonb) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.order_attempt_finish_notification(uuid,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.order_attempt_finish_notification(uuid,text,jsonb) TO commerce_order_attempt_owner;

-- Validate the final pair on either side of a mutation. River is required while
-- READY, not retained as delivery evidence after terminal job pruning. Older
-- dispatch policy rejects the new request policy before HTTP; OA2 must introduce
-- its paired C3 authority before this intent can ever dispatch.
CREATE FUNCTION inbox.check_order_payment_notification_pair() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE n inbox.order_payment_notifications%ROWTYPE; o integration.operations%ROWTYPE;
 tenant uuid; store uuid; operation uuid; notification uuid; valid boolean;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
BEGIN
 tenant:=NEW.tenant_id; store:=NEW.store_id;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true);
 IF TG_TABLE_SCHEMA='inbox' THEN notification:=NEW.id;
 ELSE operation:=NEW.id; END IF;
 SELECT x.* INTO n FROM inbox.order_payment_notifications x WHERE x.tenant_id=tenant AND x.store_id=store
  AND ((notification IS NOT NULL AND x.id=notification) OR (operation IS NOT NULL AND x.operation_id=operation));
 IF NOT FOUND THEN RAISE EXCEPTION 'notification_pair_missing' USING ERRCODE='23514'; END IF;
 IF n.operation_id IS NULL THEN
  valid:=n.state='VOID' AND n.void_stage='before_c3' AND n.dispatch_authorized_at IS NULL;
 ELSE
  SELECT x.* INTO o FROM integration.operations x WHERE x.tenant_id=tenant AND x.store_id=store AND x.id=n.operation_id;
  valid:=FOUND AND o.binding_id=n.binding_id AND o.action='meta.dm_send'
   AND o.request->>'policy'='order-attempt-notification/v1' AND o.request->>'notification_id'=n.id::text
   AND o.request->>'order_id'=n.order_id::text AND o.request->>'link_version'=n.link_version::text
   AND o.request->>'conversation_id'=n.conversation_id::text AND o.request->>'outbound_id'=n.outbound_id::text
   AND o.generation=n.worker_generation
   AND ((n.state='READY' AND o.state='READY') OR (n.state='DISPATCHING' AND o.state='DISPATCHING')
    OR (n.state='ACCEPTED' AND o.state='SUCCEEDED') OR (n.state='DELIVERY_UNKNOWN' AND o.state IN ('UNKNOWN','ACKNOWLEDGED'))
    OR (n.state='VOID' AND o.state IN ('CANCELLED','BLOCKED_POLICY','STALE_BINDING','FAILED_FINAL')));
  IF n.state='READY' THEN
   valid:=valid AND EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=o.job_id AND j.kind='external_operation_v1'
    AND j.queue='default' AND j.unique_key IS NULL AND j.args=jsonb_build_object('operation_id',o.id::text,'version',1))
    AND EXISTS(SELECT 1 FROM inbox.outbound_messages x WHERE x.tenant_id=tenant AND x.store_id=store
     AND x.id=n.outbound_id AND x.operation_id=o.id AND x.conversation_id=n.conversation_id
     AND x.kind='dm' AND x.template_id='order-pay-link/v1' AND x.template_version=1)
    AND EXISTS(SELECT 1 FROM inbox.send_secrets s WHERE s.tenant_id=tenant AND s.store_id=store AND s.operation_id=o.id);
  END IF;
 END IF;
 IF valid IS DISTINCT FROM true THEN RAISE EXCEPTION 'notification_pair_invalid' USING ERRCODE='23514'; END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN NULL;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION inbox.check_order_payment_notification_pair() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.check_order_payment_notification_pair() FROM PUBLIC;
CREATE CONSTRAINT TRIGGER order_payment_notification_pair AFTER INSERT OR UPDATE ON inbox.order_payment_notifications
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION inbox.check_order_payment_notification_pair();
CREATE CONSTRAINT TRIGGER order_payment_operation_insert_pair AFTER INSERT ON integration.operations
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
 WHEN (NEW.request->>'policy'='order-attempt-notification/v1')
 EXECUTE FUNCTION inbox.check_order_payment_notification_pair();
CREATE CONSTRAINT TRIGGER order_payment_operation_pair AFTER UPDATE ON integration.operations
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
 WHEN (OLD.request->>'policy'='order-attempt-notification/v1' OR NEW.request->>'policy'='order-attempt-notification/v1')
 EXECUTE FUNCTION inbox.check_order_payment_notification_pair();

COMMENT ON TABLE inbox.order_payment_notifications IS
 'Business outbox without new PII: encrypted display/recipient/payload are native outbound_messages/send_secrets. Initial semantic identity is server-derived from kind+attempt. NOT_REQUESTED writes nothing; refused-before-C3 VOID has no operation/job/binding fabrication. Exact native scoped order/creator provenance, immutable identity and operation reference, deferred paired states and READY exact River job. C3 dispatcher/public fragment URL consumer deferred to OA2.';
COMMENT ON FUNCTION inbox.order_attempt_prepare_notification(uuid,text) IS
 'Private C2 exact held-plan/fence and already-held order proof; current payable v1 entry, permission/window/binding/template/origin; internal encrypted thread DTO only. No browser-session/admission restart or provider action. Restores caller scope.';
COMMENT ON FUNCTION inbox.order_attempt_finish_notification(uuid,text,jsonb) IS
 'Private C2 strict artifact metadata; rederive current refusal, retain exact replay, or emit native encrypted READY operation/outbound/secret plus business outbox. Parent creates exact external_operation_v1 job in same outer Tx. New order-attempt-notification/v1 policy deliberately fails existing dispatcher closed; OA2 owns paired C3 and URL activation. No commit/provider IO or legacy buyer authority.';
