-- A report and its local reconcile job are one durable intake. The old
-- five-argument entry must not remain an unaudited ingestion path.
DROP FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb);
GRANT USAGE ON SCHEMA river TO commerce_integration_writer;
GRANT SELECT ON river.river_job TO commerce_integration_writer;

CREATE FUNCTION integration.record_payment_query(p_id uuid,p_generation bigint,p_token bytea,
 p_profile text,p_report jsonb,p_reconcile_job bigint)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; v_ref text; v_lease timestamptz; v_hash bytea;
 v_day timestamp;
BEGIN
 a:=integration.require_payment_query(p_id,p_generation,p_token,p_profile);
 IF p_report IS NULL OR jsonb_typeof(p_report)<>'object' OR octet_length(p_report::text)>2048 THEN
  RAISE EXCEPTION 'invalid payment report' USING ERRCODE='22023'; END IF;
 IF NOT p_report ?& ARRAY['MerTradeNo','TradeNo','AmountTWD','PaymentType','TradeStatus','Status','AuthType','CardInst','DataSource','CloseStatus']
 OR (SELECT count(*) FROM jsonb_object_keys(p_report)) NOT BETWEEN 10 AND 16
 OR EXISTS(SELECT 1 FROM jsonb_each(p_report) e WHERE e.key NOT IN
  ('MerTradeNo','TradeNo','AmountTWD','PaymentType','TradeStatus','Status','AuthType','CardInst','DataSource','CloseStatus',
   'CloseAmountTWD','CardRefundType','CardRefundStatus','CardRefundAmountTWD','CardRefundDay','CardRemainAmountTWD'))
 OR EXISTS(SELECT 1 FROM jsonb_each(p_report) e WHERE
  (e.key IN ('AmountTWD','CardInst','CloseAmountTWD','CardRefundAmountTWD','CardRemainAmountTWD')
    AND (jsonb_typeof(e.value)<>'number' OR e.value::text !~ '^(0|[1-9][0-9]{0,5})$'
      OR (e.key<>'CardInst' AND (e.value::text)::bigint>199999)))
  OR (e.key NOT IN ('AmountTWD','CardInst','CloseAmountTWD','CardRefundAmountTWD','CardRemainAmountTWD')
    AND jsonb_typeof(e.value)<>'string'))
 OR p_report->'AmountTWD' IS DISTINCT FROM to_jsonb(a.amount_minor/100)
 OR p_report->'CardInst' IS DISTINCT FROM '0'::jsonb
 OR p_report->>'MerTradeNo'<>a.merchant_trade_no OR a.currency<>'TWD' OR a.method_code<>'payuni_credit'
 OR p_report->>'PaymentType'<>'1' OR p_report->>'AuthType'<>'1' OR p_report->>'Status'<>'SUCCESS'
 OR p_report->>'TradeStatus' NOT IN ('0','1','2','3','4','8','9')
 OR p_report->>'DataSource' NOT IN ('A','B') OR p_report->>'CloseStatus' NOT IN ('','1','2','3','7','9')
 OR p_report->>'CardRefundType' NOT IN ('','2','3')
 OR p_report->>'CardRefundStatus' NOT IN ('','1','2','3','8')
 OR (p_report ? 'CardRefundDay' AND p_report->>'CardRefundDay'<>''
  AND p_report->>'CardRefundDay' !~
   '^[0-9]{4}-(0[1-9]|1[0-2])-([0-2][0-9]|3[01]) ([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$')
 OR (p_report->>'TradeNo'<>'' AND p_report->>'TradeNo' !~ '^[A-Za-z0-9_-]{1,64}$') THEN
  RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409'; END IF;
 IF p_report ? 'CardRefundDay' AND p_report->>'CardRefundDay'<>'' THEN
  BEGIN
   v_day:=make_timestamp(substr(p_report->>'CardRefundDay',1,4)::int,
    substr(p_report->>'CardRefundDay',6,2)::int,substr(p_report->>'CardRefundDay',9,2)::int,
    substr(p_report->>'CardRefundDay',12,2)::int,substr(p_report->>'CardRefundDay',15,2)::int,
    substr(p_report->>'CardRefundDay',18,2)::int);
   IF to_char(v_day,'YYYY-MM-DD HH24:MI:SS')<>p_report->>'CardRefundDay' THEN
    RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409'; END IF;
  EXCEPTION WHEN datetime_field_overflow THEN
   RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409';
  END;
 END IF;
 SELECT x.provider_reference,x.lease_until INTO v_ref,v_lease FROM integration.operations x WHERE x.id=p_id;
 IF v_ref<>'' AND v_ref<>p_report->>'TradeNo' THEN
  RAISE EXCEPTION 'payment reference changed' USING ERRCODE='PT409'; END IF;
 v_hash:=sha256(convert_to(p_report::text,'UTF8'));
 IF p_reconcile_job IS NULL OR p_reconcile_job<1 OR NOT EXISTS(
  SELECT 1 FROM river.river_job j WHERE j.id=p_reconcile_job AND j.kind='payment_reconcile_v1'
   AND j.state='available' AND j.attempt=0
   AND j.args=jsonb_build_object('operation_id',p_id::text,'report_hash',encode(v_hash,'hex'),'version',1)) THEN
  RAISE EXCEPTION 'payment reconcile job missing' USING ERRCODE='PT409'; END IF;
 INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,
  first_generation,report,report_hash)
 VALUES(a.tenant_id,a.store_id,a.id,'QUERY',p_profile,a.environment,p_generation,p_report,v_hash)
 ON CONFLICT(tenant_id,store_id,attempt_id,report_hash) DO NOTHING;
 PERFORM integration.complete_operation(p_id,p_generation,p_token,'UNKNOWN','payment_report_observed',p_report->>'TradeNo');
 IF clock_timestamp()>=v_lease THEN
  RAISE EXCEPTION 'payment query lease conflict' USING ERRCODE='40001'; END IF;
END $$;
ALTER FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint) TO commerce_worker;

-- The attempt is an initiation record. These rows are monotonic financial and
-- review evidence; no role receives UPDATE or DELETE on either table.
CREATE TABLE payments.facts (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,attempt_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('AUTHORIZED','CAPTURED')),
 amount_minor bigint NOT NULL CHECK(amount_minor BETWEEN 100 AND 19999900 AND amount_minor%100=0),
 currency text NOT NULL CHECK(currency='TWD'),provider_reference text NOT NULL CHECK(provider_reference ~ '^[A-Za-z0-9_-]{1,64}$'),
 connection_id uuid NOT NULL,execution_profile text NOT NULL CHECK(execution_profile IN ('PROVIDER_MOCK','SANDBOX','LIVE')),
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 source_report_hash bytea NOT NULL CHECK(octet_length(source_report_hash)=32),
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,attempt_id,kind),
 UNIQUE(tenant_id,store_id,connection_id,provider_reference,kind),
 FOREIGN KEY(tenant_id,store_id,attempt_id) REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,connection_id) REFERENCES integration.merchant_accounts(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,attempt_id,source_report_hash)
  REFERENCES payments.provider_observations(tenant_id,store_id,attempt_id,report_hash)
);
CREATE TABLE payments.review_cases (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,attempt_id uuid NOT NULL,
 reason text NOT NULL CHECK(reason IN ('CAPTURE_EVIDENCE_INCOMPLETE','REFUND_HISTORY',
  'CONFLICTING_REPORT','PAID_ALLOCATION_FAILED')),
 source_report_hash bytea NOT NULL CHECK(octet_length(source_report_hash)=32),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,attempt_id,reason),
 FOREIGN KEY(tenant_id,store_id,attempt_id,source_report_hash)
  REFERENCES payments.provider_observations(tenant_id,store_id,attempt_id,report_hash)
);
CREATE TABLE fulfillment.payment_work_items (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,order_id uuid NOT NULL,
 attempt_id uuid NOT NULL,capture_kind text NOT NULL DEFAULT 'CAPTURED' CHECK(capture_kind='CAPTURED'),
 state text NOT NULL CHECK(state IN ('READY','REVIEW_REQUIRED')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,attempt_id,capture_kind) REFERENCES payments.facts(tenant_id,store_id,attempt_id,kind)
);

DO $$ DECLARE relation_name text; BEGIN
 FOREACH relation_name IN ARRAY ARRAY['payments.facts','payments.review_cases','fulfillment.payment_work_items'] LOOP
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',relation_name);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',relation_name);
  EXECUTE format('CREATE POLICY private_writer ON %s TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)
   WITH CHECK(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation_name);
  EXECUTE format('CREATE POLICY merchant_read ON %s FOR SELECT TO commerce_runtime
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation_name);
 END LOOP;
END $$;
GRANT SELECT,INSERT ON payments.facts,payments.review_cases,fulfillment.payment_work_items TO commerce_checkout_writer;
GRANT UPDATE(state) ON fulfillment.payment_work_items TO commerce_checkout_writer;
GRANT SELECT ON payments.facts,payments.review_cases,fulfillment.payment_work_items TO commerce_runtime;
GRANT USAGE ON SCHEMA payments TO commerce_worker;
GRANT SELECT ON payments.provider_observations TO commerce_checkout_writer;
CREATE POLICY capture_observation_read ON payments.provider_observations FOR SELECT TO commerce_checkout_writer USING(true);
CREATE POLICY capture_attempt_lookup ON checkout.payment_attempts FOR SELECT TO commerce_checkout_writer USING(true);

-- Extend the single ledger path. A payment line is tied to an existing gross
-- capture fact and the original checkout reservation, never a merchant actor.
ALTER TABLE inventory.ledger ADD COLUMN payment_attempt_id uuid,ADD COLUMN payment_fact_kind text;
ALTER TABLE inventory.ledger DROP CONSTRAINT ledger_checkout_actor;
ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_checkout_actor CHECK(
 (actor_kind='MERCHANT' AND principal_id IS NOT NULL AND checkout_id IS NULL
  AND buyer_owner_id IS NULL AND buyer_session_id IS NULL AND payment_attempt_id IS NULL AND payment_fact_kind IS NULL)
 OR (actor_kind='BUYER' AND principal_id IS NULL AND checkout_id IS NOT NULL
  AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL
  AND reservation_id=checkout_id AND kind='RESERVE' AND payment_attempt_id IS NULL AND payment_fact_kind IS NULL)
 OR (actor_kind='SYSTEM_EXPIRY' AND principal_id IS NULL AND checkout_id IS NOT NULL
  AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL
  AND reservation_id=checkout_id AND kind='RELEASE' AND payment_attempt_id IS NULL AND payment_fact_kind IS NULL)
 OR (actor_kind='SYSTEM_PAYMENT' AND principal_id IS NULL AND checkout_id IS NOT NULL
  AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL AND reservation_id=checkout_id
  AND kind='ALLOCATE' AND payment_attempt_id IS NOT NULL AND payment_fact_kind='CAPTURED'
  AND operation='checkout.payment.capture' AND command_key=payment_attempt_id::text));
ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_payment_fact_fk
 FOREIGN KEY(tenant_id,store_id,payment_attempt_id,payment_fact_kind)
 REFERENCES payments.facts(tenant_id,store_id,attempt_id,kind);
ALTER TABLE inventory.ledger DROP CONSTRAINT ledger_kind_check;
ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_kind_check CHECK(kind IN ('ADJUST','RESERVE','RELEASE','ALLOCATE'));
ALTER TABLE inventory.ledger DROP CONSTRAINT ledger_check;
ALTER TABLE inventory.ledger ADD CONSTRAINT ledger_check CHECK(
 delta_unavailable=0 AND (
  (kind='ADJUST' AND delta_on_hand<>0 AND delta_reserved=0 AND delta_allocated=0
   AND reservation_id IS NULL AND length(reason)>0)
  OR (kind='RESERVE' AND delta_on_hand=0 AND delta_reserved>0 AND delta_allocated=0 AND reservation_id IS NOT NULL)
  OR (kind='RELEASE' AND delta_on_hand=0 AND delta_reserved<0 AND delta_allocated=0 AND reservation_id IS NOT NULL)
  OR (kind='ALLOCATE' AND delta_on_hand=0 AND delta_reserved<0 AND delta_allocated=-delta_reserved
   AND reservation_id IS NOT NULL)));
DROP POLICY checkout_writer_ledger ON inventory.ledger;
CREATE POLICY checkout_writer_ledger ON inventory.ledger FOR INSERT TO commerce_checkout_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND actor_kind IN ('BUYER','SYSTEM_EXPIRY','SYSTEM_PAYMENT') AND principal_id IS NULL
  AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
  AND buyer_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
CREATE FUNCTION inventory.guard_payment_ledger() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.actor_kind='SYSTEM_PAYMENT' AND NOT EXISTS(
  SELECT 1 FROM payments.facts f JOIN checkout.payment_attempts a
   ON a.tenant_id=f.tenant_id AND a.store_id=f.store_id AND a.id=f.attempt_id
   JOIN checkout.orders o ON o.tenant_id=a.tenant_id AND o.store_id=a.store_id
    AND o.owner_id=a.owner_id AND o.id=a.order_id
   WHERE f.tenant_id=NEW.tenant_id AND f.store_id=NEW.store_id
    AND f.attempt_id=NEW.payment_attempt_id AND f.kind='CAPTURED'
    AND a.order_id=NEW.checkout_id AND o.creator_session_id=NEW.buyer_session_id
    AND a.owner_id=NEW.buyer_owner_id AND f.amount_minor=a.amount_minor
    AND f.currency=a.currency) THEN
  RAISE EXCEPTION 'payment ledger mismatch' USING ERRCODE='42501'; END IF;
 IF NEW.actor_kind='SYSTEM_PAYMENT' AND NOT EXISTS(
  SELECT 1 FROM inventory.reservation_lines l WHERE l.tenant_id=NEW.tenant_id
   AND l.store_id=NEW.store_id AND l.reservation_id=NEW.reservation_id
   AND l.warehouse_id=NEW.warehouse_id AND l.sku_id=NEW.sku_id
   AND l.quantity=-NEW.delta_reserved) THEN
  RAISE EXCEPTION 'payment ledger quantity mismatch' USING ERRCODE='42501'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION inventory.guard_payment_ledger() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION inventory.guard_payment_ledger() FROM PUBLIC;
CREATE TRIGGER zz_payment_ledger_guard BEFORE INSERT ON inventory.ledger
 FOR EACH ROW EXECUTE FUNCTION inventory.guard_payment_ledger();

ALTER TABLE checkout.orders DROP CONSTRAINT orders_fulfillment_state_check;
ALTER TABLE checkout.orders ADD CONSTRAINT orders_fulfillment_state_check
 CHECK(fulfillment_state IN ('MANUAL_UNASSIGNED','CANCELLED','PAID_ALLOCATION_FAILED'));
ALTER TABLE checkout.events DROP CONSTRAINT events_action_check;
ALTER TABLE checkout.events DROP CONSTRAINT events_actor_action;
ALTER TABLE checkout.events ADD CONSTRAINT events_action_check
 CHECK(action IN ('checkout.held','checkout.expired','checkout.payment_started','checkout.payment_captured'));
ALTER TABLE checkout.events ADD CONSTRAINT events_actor_action CHECK(
 (action IN ('checkout.held','checkout.payment_started') AND actor_kind='BUYER')
 OR (action='checkout.expired' AND actor_kind='SYSTEM_EXPIRY')
 OR (action='checkout.payment_captured' AND actor_kind='SYSTEM_PAYMENT'));
ALTER TABLE checkout.events DROP CONSTRAINT events_actor_kind_check;
ALTER TABLE checkout.events ADD CONSTRAINT events_actor_kind_check
 CHECK(actor_kind IN ('BUYER','SYSTEM_EXPIRY','SYSTEM_PAYMENT'));

CREATE FUNCTION payments.apply_capture(p_attempt uuid,p_report_hash bytea)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; obs payments.provider_observations%ROWTYPE;
 op integration.operations%ROWTYPE; acct integration.merchant_accounts%ROWTYPE;
 ord checkout.orders%ROWTYPE; res inventory.reservations%ROWTYPE; ln record;
 v_auth boolean; v_capture boolean; v_refund boolean; v_conflict boolean;
 v_review boolean; v_captured boolean; v_new_capture boolean:=false; v_now timestamptz;
BEGIN
 IF p_attempt IS NULL OR p_report_hash IS NULL OR octet_length(p_report_hash)<>32 THEN
  RAISE EXCEPTION 'invalid capture input' USING ERRCODE='22023'; END IF;
 -- Only this private definer may discover scope from the frozen attempt.
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=p_attempt;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment observation unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
 PERFORM set_config('app.store_id',a.store_id::text,true);
 PERFORM set_config('app.buyer_id',a.owner_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 SELECT x.* INTO obs FROM payments.provider_observations x
  WHERE x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.attempt_id=a.id
   AND x.report_hash=p_report_hash AND x.source='QUERY';
 IF NOT FOUND OR obs.execution_profile<>a.execution_profile OR obs.environment<>a.environment
  OR obs.report_hash<>sha256(convert_to(obs.report::text,'UTF8')) THEN
  RAISE EXCEPTION 'payment observation mismatch' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO op FROM integration.operations x WHERE x.id=a.id AND x.tenant_id=a.tenant_id
  AND x.store_id=a.store_id;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.id=a.connection_id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id;
 IF op.id IS NULL OR acct.id IS NULL OR op.actor_kind<>'BUYER_PAYMENT_QUERY'
  OR op.payment_attempt_id<>a.id OR op.buyer_owner_id<>a.owner_id OR op.buyer_session_id<>a.session_id
  OR op.binding_id<>a.binding_id OR op.binding_version<>a.binding_version
  OR op.provider<>'payuni' OR op.action<>'payuni.query' OR op.purpose<>'transactional'
  OR op.external_asset_id<>acct.environment||':'||acct.account_id
  OR acct.provider<>'payuni' OR acct.environment<>a.environment OR acct.binding_id<>a.binding_id
  OR (a.execution_profile='PROVIDER_MOCK' AND a.environment<>'SANDBOX')
  OR (a.execution_profile<>'PROVIDER_MOCK' AND a.execution_profile<>a.environment)
  OR obs.report->>'MerTradeNo'<>a.merchant_trade_no
  OR obs.report->'AmountTWD' IS DISTINCT FROM to_jsonb(a.amount_minor/100)
  OR (obs.report->>'TradeNo'<>'' AND obs.report->>'TradeNo' IS DISTINCT FROM op.provider_reference)
  OR obs.report->>'Status'<>'SUCCESS' OR obs.report->>'PaymentType'<>'1'
  OR obs.report->>'AuthType'<>'1' OR obs.report->'CardInst' IS DISTINCT FROM '0'::jsonb
  OR obs.report->>'TradeStatus' NOT IN ('0','1','2','3','4','8','9')
  OR obs.report->>'DataSource' NOT IN ('A','B')
  OR obs.report->>'CloseStatus' NOT IN ('','1','2','3','7','9') THEN
  RAISE EXCEPTION 'payment provenance mismatch' USING ERRCODE='PT409'; END IF;
 -- The order is the aggregate lock; it precedes reservation and balance locks.
 SELECT x.* INTO ord FROM checkout.orders x WHERE x.tenant_id=a.tenant_id AND x.store_id=a.store_id
  AND x.owner_id=a.owner_id AND x.id=a.order_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment order unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.buyer_session_id',ord.creator_session_id::text,true);
 SELECT x.* INTO res FROM inventory.reservations x WHERE x.tenant_id=a.tenant_id
  AND x.store_id=a.store_id AND x.id=ord.id FOR UPDATE;
 IF NOT FOUND OR res.checkout_id<>ord.id OR res.buyer_owner_id<>ord.owner_id
  OR res.buyer_session_id<>ord.creator_session_id OR a.owner_id<>ord.owner_id
  OR a.currency<>ord.currency OR a.amount_minor<>ord.total_minor
 THEN
  RAISE EXCEPTION 'payment aggregate mismatch' USING ERRCODE='PT409'; END IF;
 -- Re-read operation/account after aggregate waits without locking either.
 SELECT x.* INTO op FROM integration.operations x WHERE x.id=a.id AND x.tenant_id=a.tenant_id
  AND x.store_id=a.store_id;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.id=a.connection_id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id;
 IF op.id IS NULL OR acct.id IS NULL
  OR (obs.report->>'TradeNo'<>'' AND op.provider_reference<>obs.report->>'TradeNo')
  OR op.binding_id<>a.binding_id OR op.binding_version<>a.binding_version
  OR op.external_asset_id<>acct.environment||':'||acct.account_id
  OR acct.environment<>a.environment OR acct.binding_id<>a.binding_id THEN
  RAISE EXCEPTION 'payment provenance changed' USING ERRCODE='PT409'; END IF;
 v_auth:=obs.report->>'TradeNo'<>'' AND obs.report->>'DataSource'='A'
  AND obs.report->>'TradeStatus'='1';
 v_capture:=v_auth AND obs.report->>'CloseStatus'='2'
  AND obs.report->'CloseAmountTWD' IS NOT NULL
  AND obs.report->'CloseAmountTWD'=to_jsonb(a.amount_minor/100);
 v_refund:=coalesce(obs.report->>'CardRefundType','')<>''
  OR coalesce(obs.report->>'CardRefundStatus','')<>''
  OR coalesce(obs.report->>'CardRefundDay','')<>''
  OR coalesce((obs.report->>'CardRefundAmountTWD')::bigint,0)>0
  OR (obs.report ? 'CardRemainAmountTWD' AND
   (obs.report->>'CardRemainAmountTWD')::bigint<a.amount_minor/100);
 v_conflict:=obs.report ? 'CardRemainAmountTWD' AND
  (obs.report->>'CardRemainAmountTWD')::bigint>a.amount_minor/100;
 IF obs.report->>'TradeNo'<>'' AND obs.report->>'DataSource'='A'
  AND obs.report->>'CloseStatus'='2' AND NOT v_capture THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CAPTURE_EVIDENCE_INCOMPLETE',obs.report_hash)
   ON CONFLICT DO NOTHING;
 END IF;
 IF v_refund THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'REFUND_HISTORY',obs.report_hash)
   ON CONFLICT DO NOTHING;
 END IF;
 SELECT EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id
  AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind='CAPTURED') INTO v_captured;
 IF v_conflict OR (v_captured AND obs.report->>'TradeNo'<>''
  AND obs.report->>'DataSource'='A' AND NOT v_capture) THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CONFLICTING_REPORT',obs.report_hash)
   ON CONFLICT DO NOTHING;
 END IF;
 IF v_auth THEN
  INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,
   provider_reference,connection_id,execution_profile,environment,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'AUTHORIZED',a.amount_minor,a.currency,
    op.provider_reference,a.connection_id,a.execution_profile,a.environment,obs.report_hash)
   ON CONFLICT(tenant_id,store_id,attempt_id,kind) DO NOTHING;
 END IF;
 IF v_capture THEN
  INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,
   provider_reference,connection_id,execution_profile,environment,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'CAPTURED',a.amount_minor,a.currency,
    op.provider_reference,a.connection_id,a.execution_profile,a.environment,obs.report_hash)
   ON CONFLICT(tenant_id,store_id,attempt_id,kind) DO NOTHING;
  v_new_capture:=FOUND;
  v_captured:=true;
 END IF;
 IF v_new_capture AND (res.state<>'PAYMENT_PENDING' OR ord.commercial_state<>'AWAITING_PAYMENT'
  OR a.generation<>ord.generation OR res.generation<>ord.generation) THEN
  INSERT INTO payments.review_cases(tenant_id,store_id,attempt_id,reason,source_report_hash)
   VALUES(a.tenant_id,a.store_id,a.id,'PAID_ALLOCATION_FAILED',obs.report_hash)
   ON CONFLICT DO NOTHING;
  UPDATE checkout.orders SET fulfillment_state='PAID_ALLOCATION_FAILED',updated_at=clock_timestamp()
   WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND owner_id=a.owner_id AND id=ord.id;
 END IF;
 SELECT EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id
  AND c.store_id=a.store_id AND c.attempt_id=a.id) INTO v_review;
 IF v_captured AND v_review THEN
  INSERT INTO fulfillment.payment_work_items(tenant_id,store_id,owner_id,order_id,attempt_id,state)
   VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,a.id,'REVIEW_REQUIRED')
   ON CONFLICT(tenant_id,store_id,order_id) DO UPDATE SET state='REVIEW_REQUIRED';
  RETURN;
 END IF;
 IF NOT v_capture THEN RETURN; END IF;
 -- The normal prior capture has already created its immutable work identity.
 IF EXISTS(SELECT 1 FROM fulfillment.payment_work_items w WHERE w.tenant_id=a.tenant_id
  AND w.store_id=a.store_id AND w.order_id=ord.id) THEN RETURN; END IF;
 FOR ln IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
  WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.reservation_id=ord.id
  ORDER BY l.warehouse_id,l.sku_id LOOP
  PERFORM 1 FROM inventory.lock_balance(ln.warehouse_id,ln.sku_id);
 END LOOP;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment reservation empty' USING ERRCODE='PT409'; END IF;
 FOR ln IN SELECT l.warehouse_id,l.sku_id,l.quantity FROM inventory.reservation_lines l
  WHERE l.tenant_id=a.tenant_id AND l.store_id=a.store_id AND l.reservation_id=ord.id
  ORDER BY l.warehouse_id,l.sku_id LOOP
  INSERT INTO inventory.ledger(tenant_id,store_id,warehouse_id,sku_id,kind,delta_reserved,delta_allocated,
   operation,command_key,reservation_id,principal_id,checkout_id,buyer_owner_id,buyer_session_id,
   actor_kind,payment_attempt_id,payment_fact_kind)
  VALUES(a.tenant_id,a.store_id,ln.warehouse_id,ln.sku_id,'ALLOCATE',-ln.quantity,ln.quantity,
   'checkout.payment.capture',a.id::text,ord.id,NULL,ord.id,ord.owner_id,ord.creator_session_id,
   'SYSTEM_PAYMENT',a.id,'CAPTURED');
 END LOOP;
 UPDATE inventory.reservations SET state='COMMITTED' WHERE tenant_id=a.tenant_id
  AND store_id=a.store_id AND id=ord.id AND state='PAYMENT_PENDING';
 v_now:=clock_timestamp();
 UPDATE checkout.orders SET commercial_state='CONFIRMED',updated_at=v_now
  WHERE tenant_id=a.tenant_id AND store_id=a.store_id AND owner_id=a.owner_id AND id=ord.id
   AND commercial_state='AWAITING_PAYMENT';
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
  VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,ord.creator_session_id,ord.generation,
   'checkout.payment_captured','SYSTEM_PAYMENT');
 INSERT INTO fulfillment.payment_work_items(tenant_id,store_id,owner_id,order_id,attempt_id,state)
  VALUES(a.tenant_id,a.store_id,a.owner_id,ord.id,a.id,'READY');
END $$;
ALTER FUNCTION payments.apply_capture(uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION payments.apply_capture(uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.apply_capture(uuid,bytea) TO commerce_worker;
