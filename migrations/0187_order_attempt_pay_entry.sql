-- Purpose: create one reusable order-only pay credential during the outer C2 transaction.
-- Depends on: 0171 native order provenance, 0178 execution context, 0184 native Begin.
-- Used by: commerce_order_attempt_owner after the owner/order/inventory lock prefix.
-- Invariants: I01/I04/I08/I14; original v1 material, bounded lifetime, locked validity,
-- metadata-only replay, no public link exchange/rotation, provider call or separate commit.

CREATE TABLE checkout.order_pay_entries (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL,
 order_id uuid NOT NULL, create_attempt_id uuid NOT NULL,
 link_version bigint NOT NULL CHECK(link_version>0),
 key_version text NOT NULL CHECK(key_version ~ '^[A-Za-z0-9_.-]{1,64}$'),
 token_hash bytea NOT NULL UNIQUE CHECK(octet_length(token_hash)=32),
 credential_ciphertext bytea NOT NULL CHECK(octet_length(credential_ciphertext)=71),
 issued_at timestamptz NOT NULL, expires_at timestamptz NOT NULL, revoked_at timestamptz,
 PRIMARY KEY(tenant_id,store_id,order_id,link_version),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id,create_attempt_id)
  REFERENCES checkout.orders(tenant_id,store_id,owner_id,id,create_attempt_id),
 CHECK(isfinite(issued_at) AND isfinite(expires_at)
  AND expires_at>issued_at AND expires_at<=issued_at+interval '72 hours'),
 CHECK(revoked_at IS NULL OR (isfinite(revoked_at) AND revoked_at>=issued_at))
);
CREATE UNIQUE INDEX order_pay_entries_one_unrevoked
 ON checkout.order_pay_entries(tenant_id,store_id,order_id) WHERE revoked_at IS NULL;
ALTER TABLE checkout.order_pay_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.order_pay_entries FORCE ROW LEVEL SECURITY;
REVOKE ALL ON checkout.order_pay_entries FROM PUBLIC;
GRANT SELECT,INSERT ON checkout.order_pay_entries TO commerce_checkout_writer;
CREATE POLICY order_pay_entries_read ON checkout.order_pay_entries FOR SELECT TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid);
CREATE POLICY order_pay_entries_create ON checkout.order_pay_entries FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND create_attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
  AND order_id=create_attempt_id AND link_version=1 AND revoked_at IS NULL);
COMMENT ON TABLE checkout.order_pay_entries IS
 'Native attempt-order credential versions; scoped FK targets the business order, never the replay tombstone. Only SHA256 of canonical random credential and dedicated PayEntryKeys AES-GCM replay envelope; no plaintext/contact/full-owner authority. C2 inserts v1 once; first issuance expires at min(order deadline,72h). No runtime/coordinator table grants; public exchange/copy/rotation and legacy cutover are separate units.';

-- Re-read only already-locked safety facts; do not reacquire earlier owner locks
-- after inventory. These column grants convey no buyer capability or mutation.
GRANT SELECT(tenant_id,store_id,id,active) ON buyer.owners TO commerce_checkout_writer;
CREATE POLICY order_pay_entry_owner_read ON buyer.owners FOR SELECT TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND id=nullif(current_setting('app.buyer_id',true),'')::uuid);
GRANT SELECT(id,active) ON control.tenants TO commerce_checkout_writer;

CREATE FUNCTION checkout.order_attempt_create_pay_entry(
 p_attempt uuid,p_plan_plaintext text,p_key_version text,p_token_hash bytea,p_ciphertext bytea)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_variable
DECLARE ctx jsonb; binding jsonb; tenant uuid; store uuid; buyer_id uuid; order_key bigint;
 o checkout.orders%ROWTYPE; entry checkout.order_pay_entries%ROWTYPE;
 now_at timestamptz; result jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 -- Calls the protected current-xid accessor before inspecting any credential.
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext); binding:=ctx->'binding';
 tenant:=(binding->>'tenant_id')::uuid; store:=(binding->>'store_id')::uuid;
 buyer_id:=(binding->>'buyer_owner_id')::uuid;
 IF p_key_version IS NULL OR p_key_version !~ '^[A-Za-z0-9_.-]{1,64}$'
  OR p_token_hash IS NULL OR octet_length(p_token_hash)<>32
  OR p_ciphertext IS NULL OR octet_length(p_ciphertext)<>71 THEN
  RAISE EXCEPTION 'pay_entry_invalid' USING ERRCODE='PT400'; END IF;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',binding->>'principal_id',true),set_config('app.buyer_id',buyer_id::text,true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- Require the canonical order lock retained by the caller's earlier prefix.
 -- Never acquire a missing earlier lock after inventory or disclose validity without it.
 order_key:=hashtextextended('checkout.order|'||tenant::text||'|'||store::text||'|'||p_attempt::text,0);
 IF NOT EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=pg_backend_pid() AND l.locktype='advisory'
  AND l.classid=((order_key>>32)&4294967295)::oid AND l.objid=(order_key&4294967295)::oid
  AND l.objsubid=1 AND l.mode='ExclusiveLock' AND l.granted) THEN
  RAISE EXCEPTION 'pay_entry_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO o FROM checkout.orders x WHERE x.tenant_id=tenant AND x.store_id=store
  AND x.owner_id=buyer_id AND x.id=p_attempt AND x.create_attempt_id=p_attempt
  AND x.creator_kind='order_attempt' AND x.creator_session_id IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'pay_entry_unavailable' USING ERRCODE='PT409'; END IF;
 now_at:=clock_timestamp();
 IF o.expires_at<=now_at OR o.generation<>1
  OR NOT EXISTS(SELECT 1 FROM control.tenants t WHERE t.id=tenant AND t.active)
  OR NOT EXISTS(SELECT 1 FROM control.stores s WHERE s.tenant_id=tenant AND s.id=store AND s.active)
  OR NOT EXISTS(SELECT 1 FROM buyer.owners b WHERE b.tenant_id=tenant AND b.store_id=store AND b.id=buyer_id AND b.active)
  OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=tenant AND a.store_id=store AND a.order_id=o.id)
  OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=tenant AND r.store_id=store
   AND r.id=o.id AND r.checkout_id=o.id AND r.buyer_owner_id=buyer_id AND r.buyer_session_id IS NULL
   AND r.creator_kind='order_attempt' AND r.create_attempt_id=p_attempt AND r.generation=o.generation
   AND ((o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER'
    AND r.state='HELD' AND r.expires_at>now_at
    AND EXISTS(SELECT 1 FROM checkout.bank_transfers b WHERE b.tenant_id=tenant AND b.store_id=store
     AND b.owner_id=buyer_id AND b.order_id=o.id AND b.state IN ('AWAITING','SUBMITTED','REJECTED')))
    OR (o.payment_mode='pay_at_pickup' AND o.commercial_state='CONFIRMED'
     AND o.collection_state='PENDING' AND r.state='COMMITTED')
    OR (o.payment_mode='cash_on_delivery' AND o.commercial_state='AWAITING_COLLECTION'
     AND o.collection_state='PENDING' AND r.state='COMMITTED'))) THEN
  RAISE EXCEPTION 'pay_entry_unavailable' USING ERRCODE='PT409'; END IF;
 -- Initial C2 supports precisely native Begin's three offline payment modes.
 -- Card payment is outside the existing merchant manual-order contract.
 SELECT x.* INTO entry FROM checkout.order_pay_entries x WHERE x.tenant_id=tenant AND x.store_id=store
  AND x.owner_id=buyer_id AND x.order_id=o.id AND x.create_attempt_id=p_attempt AND x.link_version=1;
 IF FOUND THEN
  IF o.source<>'merchant_manual' OR entry.revoked_at IS NOT NULL OR entry.expires_at<=now_at
   OR entry.issued_at>now_at OR entry.expires_at>o.expires_at THEN
   RAISE EXCEPTION 'pay_entry_unavailable' USING ERRCODE='PT409'; END IF;
 ELSE
  IF EXISTS(SELECT 1 FROM checkout.order_pay_entries x WHERE x.tenant_id=tenant AND x.store_id=store AND x.order_id=o.id) THEN
   RAISE EXCEPTION 'pay_entry_unavailable' USING ERRCODE='PT409'; END IF;
  INSERT INTO checkout.order_pay_entries(tenant_id,store_id,owner_id,order_id,create_attempt_id,
   link_version,key_version,token_hash,credential_ciphertext,issued_at,expires_at)
   VALUES(tenant,store,buyer_id,o.id,p_attempt,1,p_key_version,p_token_hash,p_ciphertext,
    now_at,least(o.expires_at,now_at+interval '72 hours')) RETURNING * INTO entry;
  UPDATE checkout.orders SET source='merchant_manual'
   WHERE tenant_id=tenant AND store_id=store AND owner_id=buyer_id AND id=o.id;
 END IF;
 -- The retained entry, not a new candidate, owns replay version and deadline.
 -- No bearer, digest, ciphertext, buyer fields or terminal attempt receipt escapes.
 result:=jsonb_build_object('order_id',o.id,'source','merchant_manual','link_version',entry.link_version,
  'pay_entry_expires_at',entry.expires_at);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 IF SQLSTATE='23505' THEN
  -- A candidate digest collision must not echo encrypted material or its hash.
  RAISE EXCEPTION 'pay_entry_unavailable' USING ERRCODE='PT409';
 END IF;
 RAISE;
END $$;
ALTER FUNCTION checkout.order_attempt_create_pay_entry(uuid,text,text,bytea,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_create_pay_entry(uuid,text,text,bytea,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_create_pay_entry(uuid,text,text,bytea,bytea) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_create_pay_entry(uuid,text,text,bytea,bytea) IS
 'Private C2 create/replay only: protected current-xid exact plan plus retained canonical order lock, native Begin order/stock, active owner/tenant/store and payable offline state. Inserts one v1 encrypted credential and manual source; exact original metadata replay never replaces material or extends deadline. Dedicated Go PayEntryKeys seals before this call. No new roles, runtime direct DML, legacy full-owner capability, public copy/rotation, notification/provider call, commit or terminal receipt. All caller GUCs restored.';
