-- Purpose: encrypted website contact admission, merchant reads/CAS and one existing notify outbox kind.
-- Depends on: published-origin resolver, identity.resolve_access, stores profile (0209), notify.outbox (0090/0098).
-- Used by: buyerhttp (issuer), httpapi (runtime), internal/notify's existing expiry worker.
-- Invariants: no direct runtime table grants; all PII encrypted; scope rechecked after waits; admission/outbox atomic.
CREATE ROLE commerce_store_contact_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA storefront,control,buyer,identity,notify TO commerce_store_contact_writer;
GRANT SELECT(tenant_id,store_id,origin) ON control.storefront_domains TO commerce_store_contact_writer;
CREATE POLICY contact_origin_scope ON control.storefront_domains FOR SELECT TO commerce_store_contact_writer USING(true);
COMMENT ON POLICY contact_origin_scope ON control.storefront_domains IS '0211: private definer maps the exact verified origin to tenant/store; store UUID alone is not globally unique.';
GRANT EXECUTE ON FUNCTION buyer.resolve_published_store(text),identity.resolve_access(bytea,uuid,text) TO commerce_store_contact_writer;

CREATE TABLE storefront.store_contact_messages (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL,
 command_hash bytea NOT NULL CHECK(octet_length(command_hash)=32),
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 65552),
 status text NOT NULL DEFAULT 'open' CHECK(status IN ('open','handled')),
 version bigint NOT NULL DEFAULT 0 CHECK((status='open' AND version=0) OR (status='handled' AND version=1)),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), handled_at timestamptz,
 PRIMARY KEY(tenant_id,store_id,id), UNIQUE(tenant_id,store_id,command_hash),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 CHECK((status='handled')=(handled_at IS NOT NULL))
);
ALTER TABLE storefront.store_contact_messages OWNER TO commerce_store_contact_writer;
ALTER TABLE storefront.store_contact_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.store_contact_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY contact_owner ON storefront.store_contact_messages TO commerce_store_contact_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE INDEX contact_retention ON storefront.store_contact_messages(store_id,created_at);
CREATE INDEX contact_retention_global ON storefront.store_contact_messages(created_at);
-- Only the NOLOGIN writer sees expired rows across stores, for bounded worker retention.
-- Public/merchant definers still include their exact resolved store/tenant predicates.
CREATE POLICY contact_expired ON storefront.store_contact_messages TO commerce_store_contact_writer
 USING(created_at<clock_timestamp()-interval '180 days') WITH CHECK(false);

CREATE TABLE storefront.store_contact_throttle (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, bucket bytea NOT NULL CHECK(octet_length(bucket)=32),
 window_start bigint NOT NULL, hits integer NOT NULL CHECK(hits>0),
 PRIMARY KEY(tenant_id,store_id,bucket,window_start)
);
ALTER TABLE storefront.store_contact_throttle OWNER TO commerce_store_contact_writer;
ALTER TABLE storefront.store_contact_throttle ENABLE ROW LEVEL SECURITY;
ALTER TABLE storefront.store_contact_throttle FORCE ROW LEVEL SECURITY;
CREATE POLICY contact_owner ON storefront.store_contact_throttle TO commerce_store_contact_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY contact_expired ON storefront.store_contact_throttle TO commerce_store_contact_writer
 USING(window_start<floor(extract(epoch FROM clock_timestamp())/600)::bigint) WITH CHECK(false);

CREATE FUNCTION storefront.prune_store_contacts(p_limit integer) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_count integer;
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 THEN RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 DELETE FROM storefront.store_contact_messages m WHERE (m.tenant_id,m.store_id,m.id) IN
  (SELECT old.tenant_id,old.store_id,old.id FROM storefront.store_contact_messages old
   WHERE old.created_at<clock_timestamp()-interval '180 days' ORDER BY old.created_at LIMIT p_limit FOR UPDATE SKIP LOCKED);
 GET DIAGNOSTICS v_count=ROW_COUNT;
 DELETE FROM storefront.store_contact_throttle t WHERE (t.tenant_id,t.store_id,t.bucket,t.window_start) IN
  (SELECT old.tenant_id,old.store_id,old.bucket,old.window_start FROM storefront.store_contact_throttle old
   WHERE old.window_start<floor(extract(epoch FROM clock_timestamp())/600)::bigint ORDER BY old.window_start LIMIT 200 FOR UPDATE SKIP LOCKED);
 RETURN v_count;
END $$;
ALTER FUNCTION storefront.prune_store_contacts(integer) OWNER TO commerce_store_contact_writer;
REVOKE ALL ON FUNCTION storefront.prune_store_contacts(integer) FROM PUBLIC;
GRANT USAGE ON SCHEMA storefront TO commerce_checkout_writer;
GRANT EXECUTE ON FUNCTION storefront.prune_store_contacts(integer) TO commerce_checkout_writer;

ALTER TABLE notify.outbox DROP CONSTRAINT outbox_kind_check;
ALTER TABLE notify.outbox ADD CONSTRAINT outbox_kind_check
 CHECK(kind IN ('placed','paid','shipped','cancelled','refunded','merchant_new','store_contact'));
GRANT INSERT(tenant_id,store_id,order_id,kind) ON notify.outbox TO commerce_store_contact_writer;
CREATE POLICY contact_enqueue ON notify.outbox FOR INSERT TO commerce_store_contact_writer
 WITH CHECK(kind='store_contact' AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON POLICY contact_enqueue ON notify.outbox IS '0211: only the contact notification kind, atomically with encrypted contact persistence.';

-- A NULL p_id is the locked replay lookup; both calls run in ONE issuer transaction. The store advisory lock
-- serializes distinct keys too, so the independent store and IP budgets cannot be oversubscribed by concurrency.
CREATE FUNCTION storefront.admit_store_contact(p_origin text,p_key bytea,p_id uuid,p_key_id text,p_nonce bytea,p_ciphertext bytea,p_ip bytea)
RETURNS TABLE(id uuid,tenant_id uuid,store_id uuid,key_id text,nonce bytea,ciphertext bytea,status text,version bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_store uuid; v_tenant uuid; v_window bigint; v_hits integer; v_count integer;
BEGIN
 IF p_key IS NULL OR octet_length(p_key)<>32 THEN RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF v_store IS NULL THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 SELECT d.tenant_id INTO v_tenant FROM control.storefront_domains d WHERE d.origin=p_origin AND d.store_id=v_store;
 IF v_tenant IS NULL THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('store-contact:'||v_tenant::text||':'||v_store::text,0));
 -- Fresh admission after waiting; publication may have stopped while another request held the lock.
 IF NOT EXISTS(SELECT 1 FROM buyer.resolve_published_store(p_origin) r JOIN control.storefront_domains d ON d.origin=p_origin AND d.store_id=r.store_id
  WHERE r.store_id=v_store AND d.tenant_id=v_tenant) THEN
  RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true);
 RETURN QUERY SELECT m.id,m.tenant_id,m.store_id,m.key_id,m.nonce,m.ciphertext,m.status,m.version
  FROM storefront.store_contact_messages m WHERE m.tenant_id=v_tenant AND m.store_id=v_store AND m.command_hash=p_key;
 IF FOUND THEN RETURN; END IF;
 IF p_id IS NULL THEN
  RETURN QUERY SELECT NULL::uuid,v_tenant,v_store,NULL::text,NULL::bytea,NULL::bytea,'new'::text,0::bigint;
  RETURN;
 END IF;
 IF p_key_id IS NULL OR p_key_id !~ '^[A-Za-z0-9_-]{1,64}$' OR p_nonce IS NULL OR octet_length(p_nonce)<>12 OR
  p_ciphertext IS NULL OR octet_length(p_ciphertext) NOT BETWEEN 17 AND 65552 OR p_ip IS NULL OR octet_length(p_ip)<>32 THEN
  RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 v_window:=floor(extract(epoch FROM clock_timestamp())/600)::bigint;
 SELECT t.hits INTO v_hits FROM storefront.store_contact_throttle t
  WHERE t.tenant_id=v_tenant AND t.store_id=v_store AND t.bucket=p_ip AND t.window_start=v_window;
 SELECT coalesce(sum(t.hits),0) INTO v_count FROM storefront.store_contact_throttle t WHERE t.tenant_id=v_tenant AND t.store_id=v_store AND t.window_start=v_window;
 IF coalesce(v_hits,0)>=5 OR v_count>=100 THEN RAISE EXCEPTION 'rate limited' USING ERRCODE='PT429'; END IF;
 INSERT INTO storefront.store_contact_throttle(tenant_id,store_id,bucket,window_start,hits) VALUES(v_tenant,v_store,p_ip,v_window,1)
  ON CONFLICT ON CONSTRAINT store_contact_throttle_pkey DO UPDATE SET hits=storefront.store_contact_throttle.hits+1;
 -- Bounded, store-local retention work, in the same ongoing admission path (no new scheduler).
 DELETE FROM storefront.store_contact_throttle t WHERE t.tenant_id=v_tenant AND t.store_id=v_store AND t.window_start<v_window;
 DELETE FROM storefront.store_contact_messages m WHERE (m.tenant_id,m.store_id,m.id) IN
  (SELECT old.tenant_id,old.store_id,old.id FROM storefront.store_contact_messages old
   WHERE old.tenant_id=v_tenant AND old.store_id=v_store AND old.created_at<clock_timestamp()-interval '180 days' ORDER BY old.created_at LIMIT 100);
 INSERT INTO storefront.store_contact_messages(tenant_id,store_id,id,command_hash,key_id,nonce,ciphertext)
  VALUES(v_tenant,v_store,p_id,p_key,p_key_id,p_nonce,p_ciphertext);
 INSERT INTO notify.outbox(tenant_id,store_id,order_id,kind) VALUES(v_tenant,v_store,p_id,'store_contact');
 RETURN QUERY SELECT m.id,m.tenant_id,m.store_id,m.key_id,m.nonce,m.ciphertext,m.status,m.version
  FROM storefront.store_contact_messages m WHERE m.tenant_id=v_tenant AND m.store_id=v_store AND m.id=p_id;
END $$;
ALTER FUNCTION storefront.admit_store_contact(text,bytea,uuid,text,bytea,bytea,bytea) OWNER TO commerce_store_contact_writer;
REVOKE ALL ON FUNCTION storefront.admit_store_contact(text,bytea,uuid,text,bytea,bytea,bytea) FROM PUBLIC;
GRANT USAGE ON SCHEMA storefront TO commerce_buyer_issuer;
GRANT EXECUTE ON FUNCTION storefront.admit_store_contact(text,bytea,uuid,text,bytea,bytea,bytea) TO commerce_buyer_issuer;

CREATE FUNCTION storefront.contact_access(p_hash bytea,p_store uuid,p_permission text) RETURNS uuid
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_permission NOT IN ('inbox:read','inbox:reply') THEN
  RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,p_permission);
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' OR current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text OR
  current_setting('app.store_id',true) IS DISTINCT FROM p_store::text OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN s.tenant_id;
END $$;
ALTER FUNCTION storefront.contact_access(bytea,uuid,text) OWNER TO commerce_store_contact_writer;
REVOKE ALL ON FUNCTION storefront.contact_access(bytea,uuid,text) FROM PUBLIC;

CREATE FUNCTION storefront.read_store_contacts(p_hash bytea,p_store uuid,p_id uuid,p_after uuid,p_limit integer)
RETURNS TABLE(id uuid,tenant_id uuid,store_id uuid,key_id text,nonce bytea,ciphertext bytea,status text,version bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid;
BEGIN
 v_tenant:=storefront.contact_access(p_hash,p_store,'inbox:read');
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101 THEN RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 RETURN QUERY SELECT m.id,m.tenant_id,m.store_id,m.key_id,m.nonce,m.ciphertext,m.status,m.version
  FROM storefront.store_contact_messages m WHERE m.tenant_id=v_tenant AND m.store_id=p_store
  AND (p_id IS NULL OR m.id=p_id) AND (p_after IS NULL OR m.id>p_after) ORDER BY m.id LIMIT p_limit;
END $$;
ALTER FUNCTION storefront.read_store_contacts(bytea,uuid,uuid,uuid,integer) OWNER TO commerce_store_contact_writer;
REVOKE ALL ON FUNCTION storefront.read_store_contacts(bytea,uuid,uuid,uuid,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION storefront.read_store_contacts(bytea,uuid,uuid,uuid,integer) TO commerce_runtime;

CREATE FUNCTION storefront.handle_store_contact(p_hash bytea,p_store uuid,p_id uuid,p_expected bigint)
RETURNS TABLE(id uuid,status text,version bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_head bigint;
BEGIN
 v_tenant:=storefront.contact_access(p_hash,p_store,'inbox:reply');
 IF p_id IS NULL OR p_expected IS NULL OR p_expected<0 THEN RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 SELECT m.version INTO v_head FROM storefront.store_contact_messages m WHERE m.tenant_id=v_tenant AND m.store_id=p_store AND m.id=p_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 PERFORM storefront.contact_access(p_hash,p_store,'inbox:reply');
 IF v_head<>p_expected OR v_head<>0 THEN RAISE EXCEPTION 'conflict' USING ERRCODE='PT409'; END IF;
 RETURN QUERY UPDATE storefront.store_contact_messages m SET status='handled',version=1,handled_at=clock_timestamp()
  WHERE m.tenant_id=v_tenant AND m.store_id=p_store AND m.id=p_id RETURNING m.id,m.status,m.version;
END $$;
ALTER FUNCTION storefront.handle_store_contact(bytea,uuid,uuid,bigint) OWNER TO commerce_store_contact_writer;
REVOKE ALL ON FUNCTION storefront.handle_store_contact(bytea,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION storefront.handle_store_contact(bytea,uuid,uuid,bigint) TO commerce_runtime;
COMMENT ON TABLE storefront.store_contact_messages IS '0211: AES-GCM PII envelope; 180-day application retention, bounded cleanup on admissions and every notify worker claim (including dormant stores). Not a legal retention assertion. No ordinary runtime table access.';
COMMENT ON TABLE storefront.store_contact_throttle IS '0211: five per hashed IP and 100 per store in fixed 600-second windows; replay precedes throttle. Expired windows removed on admission.';

-- Add only the new kind to the existing claim loop; retain 0098 fairness, 0107 COD snapshots,
-- 0143 suspension, shared caps, claim/record/UNKNOWN, staleness and retry semantics verbatim.
GRANT SELECT(contact_email) ON control.stores TO commerce_checkout_writer;
DO $patch$
DECLARE v_fn text; v_needle text:='  SELECT k.owner_id,k.buyer_email'; v_branch text;
BEGIN
 v_fn:=pg_get_functiondef('notify.claim_batch(integer,integer,integer)'::regprocedure);
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) OR
  (length(v_fn)-length(replace(v_fn,' -- UTC+8 day start','')))<>length(' -- UTC+8 day start') OR
  (length(v_fn)-length(replace(v_fn,'v_locale text;','')))<>length('v_locale text;') THEN
  RAISE EXCEPTION 'notify.claim_batch has an unexpected shape for contact notification'; END IF;
 v_branch:=$branch$
  IF c.kind='store_contact' THEN
   PERFORM set_config('app.tenant_id',c.tenant_id::text,true),set_config('app.store_id',c.store_id::text,true);
   SELECT st.name,st.contact_email INTO v_store_name,v_contact_email FROM control.stores st WHERE st.tenant_id=c.tenant_id AND st.id=c.store_id;
   IF v_contact_email IS NULL OR octet_length(v_contact_email) NOT BETWEEN 3 AND 254 OR
    v_contact_email !~ '^[^[:space:]<>"@]+@[^[:space:]<>"@]+$' OR v_contact_email ~ '[[:cntrl:]]' THEN
    UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_recipient',updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind;
    CONTINUE;
   END IF;
   v_batch:=gen_random_uuid();
   UPDATE notify.outbox SET state='SENDING',attempts=attempts+1,claimed_at=v_now,batch_id=v_batch,updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind;
   v_used:=v_used+1;
   v_out:=v_out||jsonb_build_array(jsonb_build_object('batch_id',v_batch,'kind','store_contact','order_id',c.order_id,
    'store_name',v_store_name,'to',jsonb_build_array(v_contact_email)));
   CONTINUE;
  END IF;
$branch$;
 v_fn:=replace(v_fn,'v_locale text;','v_locale text; v_contact_email text;');
 v_fn:=replace(v_fn,' -- UTC+8 day start',E' PERFORM storefront.prune_store_contacts(100);\n -- UTC+8 day start');
 EXECUTE replace(v_fn,v_needle,v_branch||v_needle);
END $patch$;
ALTER FUNCTION notify.claim_batch(integer,integer,integer) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION notify.claim_batch(integer,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.claim_batch(integer,integer,integer) TO commerce_expiry_worker;
