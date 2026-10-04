-- Post-River 0018 (T21-02/T21-03, unit worker-authority-split): finish moving commerce_worker's privileges for the
-- objects that only exist after the post-River phase, and prove commerce_worker is empty.
--
-- Owns: EXECUTE of integration.payment_queue_ready / record_stripe_observation / load_stripe_signal /
-- record_stripe_refund_observation / record_stripe_charge_observation (payment authorities) and
-- checkout.expiry_queue_ready (expiry authority); the ads-lane River guard re-pointed from commerce_worker to
-- commerce_ads_worker (static forward copy of post_river/0015; only the two pg_has_role tests changed); and the closing
-- assertion that commerce_worker holds no ACL entry anywhere.
-- Non-goals: no new object; River lifecycle grants are re-asserted by migrations/migrate.go on every Apply (they target
-- tables that upstream River migrations create), not here. Migration 0096 owns roles, lanes and the numbered-phase grants.
-- Depends on: migration 0096 (roles, helpers), post_river/0001,0002,0012,0013,0015,0016.

REVOKE EXECUTE ON FUNCTION
 integration.payment_queue_ready(),checkout.expiry_queue_ready(),
 integration.record_stripe_observation(uuid,bigint,bytea,text,jsonb,bigint,text),
 integration.load_stripe_signal(uuid,bigint,bytea,text,bigint,uuid),
 integration.record_stripe_refund_observation(uuid,bigint,bytea,text,jsonb,bigint),
 integration.record_stripe_charge_observation(uuid,bigint,bytea,text,jsonb,bigint)
 FROM commerce_worker;
GRANT EXECUTE ON FUNCTION
 integration.payment_queue_ready(),
 integration.record_stripe_observation(uuid,bigint,bytea,text,jsonb,bigint,text),
 integration.load_stripe_signal(uuid,bigint,bytea,text,bigint,uuid),
 integration.record_stripe_refund_observation(uuid,bigint,bytea,text,jsonb,bigint),
 integration.record_stripe_charge_observation(uuid,bigint,bytea,text,jsonb,bigint)
 TO commerce_payment_worker,commerce_payment_live;
GRANT EXECUTE ON FUNCTION checkout.expiry_queue_ready() TO commerce_expiry_worker;

CREATE OR REPLACE FUNCTION integration.guard_ads_job() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_periodic constant text[]:=ARRAY['ads_publish_advance_v1','ads_insights_plan_v1','ads_oauth_purge_v1','capi_purchase_sweep_v1'];
BEGIN
 IF NEW.queue IS DISTINCT FROM 'ads' AND NOT (NEW.kind=ANY(v_periodic))
  AND NOT (TG_OP='UPDATE' AND (OLD.queue='ads' OR OLD.kind=ANY(v_periodic))) THEN
  RETURN NEW;                       -- not an ads job: the default lane and every other family are unchanged
 END IF;
 IF TG_OP='UPDATE' THEN
  -- River's own state transitions (fetch, complete, snooze, retry) are allowed; identity never changes.
  IF NEW.kind IS DISTINCT FROM OLD.kind OR NEW.args IS DISTINCT FROM OLD.args OR NEW.queue IS DISTINCT FROM OLD.queue
   OR NEW.unique_key IS DISTINCT FROM OLD.unique_key THEN
   RAISE EXCEPTION 'immutable ads job identity' USING ERRCODE='22023';
  END IF;
  RETURN NEW;
 END IF;
 -- The migration owner and fixtures are superusers (pg_has_role is true for them); every application login is judged.
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname=session_user AND rolsuper) THEN RETURN NEW; END IF;
 IF NEW.queue<>'ads' OR NEW.unique_key IS NOT NULL OR NEW.args IS NULL OR jsonb_typeof(NEW.args)<>'object' THEN
  RAISE EXCEPTION 'invalid ads job' USING ERRCODE='22023';
 END IF;
 IF NEW.kind='external_operation_v1' THEN
  IF NEW.priority NOT IN (1,3) OR NEW.args->>'operation_id' IS NULL
   OR NEW.args->>'operation_id' !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
   OR NEW.args IS DISTINCT FROM jsonb_build_object('operation_id',NEW.args->>'operation_id','version',1)
   OR NOT (pg_has_role(session_user,'commerce_runtime','MEMBER') OR pg_has_role(session_user,'commerce_ads_worker','MEMBER')) THEN
   RAISE EXCEPTION 'invalid ads job' USING ERRCODE='22023';
  END IF;
 ELSIF NEW.kind=ANY(v_periodic) THEN
  IF NEW.priority NOT BETWEEN 1 AND 4 OR NEW.args<>'{}'::jsonb OR NOT pg_has_role(session_user,'commerce_ads_worker','MEMBER') THEN
   RAISE EXCEPTION 'invalid ads job' USING ERRCODE='22023';
  END IF;
 ELSE
  RAISE EXCEPTION 'invalid ads job' USING ERRCODE='22023';   -- only the four periodic kinds and external_operation_v1 ride queue ads
 END IF;
 RETURN NEW;
END $$;

-- COMMENT ON (PROCESS §5): rewrite the shared role name in the comments of the functions moved above (see 0096).
DO $$
DECLARE r record; v_comment text;
BEGIN
 FOR r IN SELECT * FROM (VALUES
  ('integration.payment_queue_ready()','commerce_payment_worker/commerce_payment_live'),
  ('checkout.expiry_queue_ready()','commerce_expiry_worker'),
  ('integration.record_stripe_observation(uuid,bigint,bytea,text,jsonb,bigint,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.load_stripe_signal(uuid,bigint,bytea,text,bigint,uuid)','commerce_payment_worker/commerce_payment_live'),
  ('integration.record_stripe_refund_observation(uuid,bigint,bytea,text,jsonb,bigint)','commerce_payment_worker/commerce_payment_live'),
  ('integration.record_stripe_charge_observation(uuid,bigint,bytea,text,jsonb,bigint)','commerce_payment_worker/commerce_payment_live'),
  ('integration.guard_ads_job()','commerce_ads_worker')
 ) AS t(sig,role_text) LOOP
  v_comment:=obj_description(to_regprocedure(r.sig),'pg_proc');
  IF v_comment LIKE '%commerce\_worker%' THEN
   EXECUTE format('COMMENT ON FUNCTION %s IS %L',r.sig,replace(v_comment,'commerce_worker',r.role_text));
  END IF;
 END LOOP;
END $$;

-- Closing gate: commerce_worker is an empty legacy role. Explicit ACL entries only (PUBLIC grants are not its own),
-- covering schemas, relations/sequences, columns and functions; a stray later GRANT in an upgrade path fails the migration.
DO $$
DECLARE v_left text;
BEGIN
 SELECT string_agg(x.what,', ' ORDER BY x.what) INTO v_left FROM (
  SELECT 'schema '||n.nspname AS what FROM pg_namespace n, aclexplode(n.nspacl) a
   WHERE a.grantee='commerce_worker'::regrole
  UNION ALL SELECT 'relation '||c.oid::regclass::text FROM pg_class c, aclexplode(c.relacl) a
   WHERE a.grantee='commerce_worker'::regrole
  UNION ALL SELECT 'column '||c.oid::regclass::text||'.'||t.attname FROM pg_class c JOIN pg_attribute t ON t.attrelid=c.oid, aclexplode(t.attacl) a
   WHERE a.grantee='commerce_worker'::regrole
  UNION ALL SELECT 'function '||p.oid::regprocedure::text FROM pg_proc p, aclexplode(p.proacl) a
   WHERE a.grantee='commerce_worker'::regrole) x;
 IF v_left IS NOT NULL THEN
  RAISE EXCEPTION 'commerce_worker still holds: %',v_left;
 END IF;
END $$;
