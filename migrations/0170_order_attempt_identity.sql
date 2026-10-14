-- Purpose: durable principal/store order-attempt identity, generation and subject fences.
-- Depends on: identity memberships, control stores, buyer owners and the ops schema.
-- Used by: OA-1 admission/execution definers and their domain-owned transaction helpers.
-- Invariants: order-attempt-v1 §§1–3; no runtime or privacy-writer direct table authority;
--   transport keys/generations stay unique after settlement; subjects exist only while pending.
-- This migration supplies storage only. It does not enable admission or replace old writers.

CREATE ROLE commerce_order_attempt_owner NOLOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS
 NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA ops TO commerce_order_attempt_owner;

-- Business provenance has a different retention basis from replay tombstones. Domain
-- FKs reference this row, not the replay record. It deliberately contains no scope,
-- generation, principal, request/key/plan hash, receipt or timestamp, so deleting the
-- replay record cannot leave a join from permanent scope-denial markers to an order.
CREATE TABLE ops.order_attempt_creators (
 attempt_id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 buyer_owner_id uuid NOT NULL,
 UNIQUE(tenant_id,store_id,buyer_owner_id,attempt_id),
 FOREIGN KEY(tenant_id,store_id,buyer_owner_id) REFERENCES buyer.owners(tenant_id,store_id,id)
  DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE ops.order_attempt_scopes (
 scope_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 principal_id uuid NOT NULL,
 operation_family text NOT NULL DEFAULT 'merchant_order'
  CHECK(operation_family='merchant_order'),
 current_generation bigint NOT NULL DEFAULT 0 CHECK(current_generation>=0),
 retired_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(tenant_id,store_id,scope_id,operation_family),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
-- A1: retired identities remain replay-addressable but cannot block a fresh
-- activation. scope_id remains the permanent identity, never an alias to a new row.
CREATE UNIQUE INDEX order_attempt_one_active_scope ON ops.order_attempt_scopes
 (tenant_id,store_id,principal_id,operation_family) WHERE retired_at IS NULL;

CREATE TABLE ops.order_attempts (
 attempt_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 scope_id uuid NOT NULL,
 operation_family text NOT NULL DEFAULT 'merchant_order'
  CHECK(operation_family='merchant_order'),
 intent_generation bigint NOT NULL CHECK(intent_generation>0),
 transport_key_digest bytea NOT NULL CHECK(octet_length(transport_key_digest)=32),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 request_schema_version integer NOT NULL CHECK(request_schema_version=1),
 -- C1 inserts its candidate before owner/domain reads, then seals it in the same Tx.
 -- The deferred trigger below makes an unfinished candidate impossible to commit.
 admission_finished boolean NOT NULL DEFAULT false,
 facts_digest bytea CHECK(octet_length(facts_digest)=32),
 plan_digest bytea CHECK(octet_length(plan_digest)=32),
 plan_ciphertext bytea,
 plan_key_version text,
 buyer_owner_id uuid NOT NULL,
 admission_provenance jsonb NOT NULL CHECK(jsonb_typeof(admission_provenance)='object'
  AND octet_length(admission_provenance::text)<=16384),
 admitted_at timestamptz NOT NULL,
 execute_by timestamptz NOT NULL,
 state text NOT NULL CHECK(state IN ('PENDING','COMMITTED','ABORTED')),
 order_id uuid,
 receipt jsonb,
 settled_at timestamptz,
 execution_job_id bigint CHECK(execution_job_id>0),
 UNIQUE(tenant_id,operation_family,transport_key_digest),
 UNIQUE(scope_id,intent_generation),
 UNIQUE(tenant_id,store_id,buyer_owner_id,attempt_id),
 UNIQUE(tenant_id,store_id,attempt_id),
 FOREIGN KEY(tenant_id,store_id,scope_id,operation_family)
  REFERENCES ops.order_attempt_scopes(tenant_id,store_id,scope_id,operation_family),
 FOREIGN KEY(tenant_id,store_id,buyer_owner_id,attempt_id)
  REFERENCES ops.order_attempt_creators(tenant_id,store_id,buyer_owner_id,attempt_id),
 CHECK(execute_by=admitted_at+interval '15 minutes'),
 CHECK((NOT admission_finished AND state='PENDING' AND order_id IS NULL AND receipt IS NULL
   AND settled_at IS NULL AND plan_digest IS NULL AND plan_ciphertext IS NULL AND plan_key_version IS NULL)
  OR (admission_finished AND facts_digest IS NOT NULL AND plan_digest IS NOT NULL
   AND ((state='PENDING' AND order_id IS NULL AND receipt IS NULL AND settled_at IS NULL
   AND plan_ciphertext IS NOT NULL AND octet_length(plan_ciphertext) BETWEEN 29 AND 1048604
   AND plan_key_version IS NOT NULL AND plan_key_version ~ '^[A-Za-z0-9_.-]{1,64}$')
  OR (state IN ('COMMITTED','ABORTED') AND plan_ciphertext IS NULL AND plan_key_version IS NULL
   AND receipt IS NOT NULL AND jsonb_typeof(receipt)='object' AND octet_length(receipt::text)<=16384
   AND settled_at IS NOT NULL AND settled_at>=admitted_at
   AND coalesce(receipt->>'state'=state AND receipt->>'attempt_id'=attempt_id::text
    AND (receipt->>'intent_generation')::bigint=intent_generation,false)
   AND ((state='COMMITTED' AND order_id IS NOT NULL AND coalesce(receipt->>'order_id'=order_id::text,false))
    OR (state='ABORTED' AND order_id IS NULL AND NOT receipt ? 'order_id'))))))
);
CREATE UNIQUE INDEX order_attempt_one_pending ON ops.order_attempts(scope_id) WHERE state='PENDING';
CREATE INDEX order_attempt_sweep ON ops.order_attempts(admitted_at,attempt_id) WHERE state='PENDING';
CREATE INDEX order_attempt_plan_key ON ops.order_attempts(plan_key_version) WHERE state='PENDING';

CREATE TABLE ops.order_attempt_subjects (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 subject_kind text NOT NULL CHECK(subject_kind IN ('customer','buyer_owner','private_owner')),
 subject_id uuid NOT NULL,
 attempt_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,store_id,subject_kind,subject_id,attempt_id),
 FOREIGN KEY(tenant_id,store_id,attempt_id)
  REFERENCES ops.order_attempts(tenant_id,store_id,attempt_id)
);
CREATE INDEX order_attempt_subject_settlement ON ops.order_attempt_subjects(attempt_id);

ALTER TABLE ops.order_attempt_scopes OWNER TO commerce_order_attempt_owner;
ALTER TABLE ops.order_attempts OWNER TO commerce_order_attempt_owner;
ALTER TABLE ops.order_attempt_subjects OWNER TO commerce_order_attempt_owner;
ALTER TABLE ops.order_attempt_creators OWNER TO commerce_order_attempt_owner;
ALTER TABLE ops.order_attempt_scopes ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_scopes FORCE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempts FORCE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_subjects ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_subjects FORCE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_creators ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_creators FORCE ROW LEVEL SECURITY;

-- The coordinator authenticates before setting scope; workers re-establish it from the
-- locked attempt. No caller role can use these policies because no direct grants exist.
CREATE POLICY order_attempt_scope_owner ON ops.order_attempt_scopes TO commerce_order_attempt_owner
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY order_attempt_owner ON ops.order_attempts TO commerce_order_attempt_owner
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY order_attempt_subject_owner ON ops.order_attempt_subjects TO commerce_order_attempt_owner
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY order_attempt_creator_owner ON ops.order_attempt_creators TO commerce_order_attempt_owner
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);

-- Job args contain only attempt_id/generation. A real claims-worker session may
-- discover scope through a coordinator entry before its first locking statement.
-- UPDATE/DELETE still require the validated scoped policy; no caller gets SELECT.
CREATE POLICY order_attempt_worker_locator ON ops.order_attempts FOR SELECT TO commerce_order_attempt_owner
 USING(pg_has_role(session_user,'commerce_claims_worker','MEMBER'));

REVOKE ALL ON ops.order_attempt_scopes,ops.order_attempts,ops.order_attempt_subjects FROM PUBLIC;
REVOKE ALL ON ops.order_attempt_creators FROM PUBLIC;
COMMENT ON TABLE ops.order_attempt_creators IS
 'order-attempt-v1 §§2–3: immutable business-domain attempt creator identity, retained with the order independently of replay tombstones; no scope/generation/hash/receipt mapping.';
COMMENT ON TABLE ops.order_attempt_scopes IS
 'order-attempt-v1 §2: server-resolved principal/store merchant-order CAS; only coordinator definers access rows.';
COMMENT ON TABLE ops.order_attempts IS
 'order-attempt-v1 §§2–3: immutable all-state key/generation/owner bindings; encrypted pending plan; terminal receipt without bearer/PII.';
COMMENT ON TABLE ops.order_attempt_subjects IS
 'order-attempt-v1 §§2–3: exact pending subject lookup for admission/erasure barriers; delete in settlement Tx, never retained as a replay index.';
COMMENT ON COLUMN ops.order_attempts.execution_job_id IS
 'Current order_attempt_v1 job pointer. River lifecycle may reap jobs; the sweeper repairs this pointer under the attempt fence, not an ON DELETE cascade.';

-- C2 never modifies the scope. This trigger also protects against accidental regression
-- in a future admission helper; a retired scope cannot be silently reactivated.
CREATE FUNCTION ops.guard_order_attempt_scope() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE prior_tenant text; prior_store text; pending boolean;
BEGIN
 IF ROW(NEW.scope_id,NEW.tenant_id,NEW.store_id,NEW.principal_id,NEW.operation_family,NEW.created_at)
  IS DISTINCT FROM ROW(OLD.scope_id,OLD.tenant_id,OLD.store_id,OLD.principal_id,OLD.operation_family,OLD.created_at)
  OR NEW.current_generation<OLD.current_generation
  OR (OLD.retired_at IS NOT NULL AND NEW IS DISTINCT FROM OLD) THEN
  RAISE EXCEPTION 'order attempt scope is immutable' USING ERRCODE='PT409';
 END IF;
 IF OLD.retired_at IS NULL AND NEW.retired_at IS NOT NULL THEN
  -- A lifecycle writer locks principal before scope. This trigger already owns
  -- the scope row lock, so it must not introduce a reverse principal wait.
  -- The trusted row establishes RLS even after an entry restored caller GUCs.
  prior_tenant:=current_setting('app.tenant_id',true);
  prior_store:=current_setting('app.store_id',true);
  PERFORM set_config('app.tenant_id',OLD.tenant_id::text,true);
  PERFORM set_config('app.store_id',OLD.store_id::text,true);
  SELECT EXISTS(SELECT 1 FROM ops.order_attempts a WHERE a.scope_id=OLD.scope_id
   AND a.tenant_id=OLD.tenant_id AND a.store_id=OLD.store_id AND a.state='PENDING') INTO pending;
  PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
  PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
  IF pending THEN
   RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409';
  END IF;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_scope() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_scope() FROM PUBLIC;
COMMENT ON FUNCTION ops.guard_order_attempt_scope() IS
 'Internal trigger only, order-attempt-v1 §2/A1: preserve scope identity and monotonic CAS generation; refuse retirement while PENDING; no external EXECUTE.';
CREATE TRIGGER order_attempt_scope_immutable BEFORE UPDATE ON ops.order_attempt_scopes
 FOR EACH ROW EXECUTE FUNCTION ops.guard_order_attempt_scope();

CREATE FUNCTION ops.guard_order_attempt_creator() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW IS DISTINCT FROM OLD THEN
  RAISE EXCEPTION 'order attempt creator is immutable' USING ERRCODE='PT409';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_creator() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_creator() FROM PUBLIC;
COMMENT ON FUNCTION ops.guard_order_attempt_creator() IS
 'Internal trigger only, order-attempt-v1 §2: business creator identity cannot move to a different owner or attempt.';
CREATE TRIGGER order_attempt_creator_immutable BEFORE UPDATE ON ops.order_attempt_creators
 FOR EACH ROW EXECUTE FUNCTION ops.guard_order_attempt_creator();

CREATE FUNCTION ops.guard_order_attempt_identity() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE prior_tenant text; prior_store text;
BEGIN
 IF ROW(NEW.attempt_id,NEW.tenant_id,NEW.store_id,NEW.scope_id,NEW.operation_family,NEW.intent_generation,
  NEW.transport_key_digest,NEW.request_hash,NEW.request_schema_version,NEW.buyer_owner_id,NEW.admitted_at,NEW.execute_by)
  IS DISTINCT FROM ROW(OLD.attempt_id,OLD.tenant_id,OLD.store_id,OLD.scope_id,OLD.operation_family,OLD.intent_generation,
  OLD.transport_key_digest,OLD.request_hash,OLD.request_schema_version,OLD.buyer_owner_id,OLD.admitted_at,OLD.execute_by)
  OR (OLD.admission_finished AND ROW(NEW.admission_finished,NEW.facts_digest,NEW.plan_digest,NEW.admission_provenance)
   IS DISTINCT FROM ROW(OLD.admission_finished,OLD.facts_digest,OLD.plan_digest,OLD.admission_provenance))
  OR (OLD.state<>'PENDING' AND NEW IS DISTINCT FROM OLD)
  OR (OLD.admission_finished AND NEW.state='PENDING' AND ROW(NEW.plan_ciphertext,NEW.plan_key_version)
   IS DISTINCT FROM ROW(OLD.plan_ciphertext,OLD.plan_key_version)) THEN
  RAISE EXCEPTION 'order attempt identity is immutable' USING ERRCODE='PT409';
 END IF;
 IF NEW.state<>'PENDING' THEN
  -- This is a trusted trigger row, not caller-supplied scope. The entry may have
  -- already restored its GUCs; FORCE RLS must not silently hide its children.
  prior_tenant:=current_setting('app.tenant_id',true);
  prior_store:=current_setting('app.store_id',true);
  PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true);
  PERFORM set_config('app.store_id',NEW.store_id::text,true);
  DELETE FROM ops.order_attempt_subjects WHERE attempt_id=NEW.attempt_id
   AND tenant_id=NEW.tenant_id AND store_id=NEW.store_id;
  PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
  PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_identity() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_identity() FROM PUBLIC;
COMMENT ON FUNCTION ops.guard_order_attempt_identity() IS
 'Internal trigger only, order-attempt-v1 §§2–3: immutable request/plan/owner/deadline, terminal monotonicity and same-Tx subject cleanup.';
CREATE TRIGGER order_attempt_identity_immutable BEFORE UPDATE ON ops.order_attempts
 FOR EACH ROW EXECUTE FUNCTION ops.guard_order_attempt_identity();

CREATE FUNCTION ops.check_order_attempt_admission() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE finished boolean; row_exists boolean; prior_tenant text; prior_store text;
BEGIN
 -- Re-read the final row: deferred INSERT and UPDATE trigger events contain old
 -- candidate versions, not necessarily the version being committed.
 -- Deferred execution happens after the entry restores its caller context.
 -- Establish scope from the authenticated row, then restore it before returning.
 prior_tenant:=current_setting('app.tenant_id',true);
 prior_store:=current_setting('app.store_id',true);
 PERFORM set_config('app.tenant_id',NEW.tenant_id::text,true);
 PERFORM set_config('app.store_id',NEW.store_id::text,true);
 SELECT admission_finished INTO finished FROM ops.order_attempts WHERE attempt_id=NEW.attempt_id
  AND tenant_id=NEW.tenant_id AND store_id=NEW.store_id;
 row_exists:=FOUND;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 IF row_exists AND NOT finished THEN
  RAISE EXCEPTION 'order attempt admission is unfinished' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION ops.check_order_attempt_admission() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.check_order_attempt_admission() FROM PUBLIC;
COMMENT ON FUNCTION ops.check_order_attempt_admission() IS
 'Internal deferred trigger, order-attempt-v1 C1: a candidate exists only within admission Tx; incomplete plan sealing cannot commit.';
CREATE CONSTRAINT TRIGGER order_attempt_admission_finished AFTER INSERT OR UPDATE ON ops.order_attempts
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION ops.check_order_attempt_admission();

-- A child can neither revive a terminal subject index nor rewrite the admitted set.
-- FOR KEY SHARE avoids taking an earlier lock level during same-Tx C2 deletion.
CREATE FUNCTION ops.guard_order_attempt_subject() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE;
BEGIN
 IF TG_OP='UPDATE' THEN
  RAISE EXCEPTION 'order attempt subject is immutable' USING ERRCODE='PT409';
 END IF;
 SELECT * INTO a FROM ops.order_attempts WHERE attempt_id=NEW.attempt_id
  AND tenant_id=NEW.tenant_id AND store_id=NEW.store_id FOR KEY SHARE;
 IF NOT FOUND OR a.state<>'PENDING' THEN
  RAISE EXCEPTION 'order attempt subject is not pending' USING ERRCODE='PT409';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_subject() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_subject() FROM PUBLIC;
COMMENT ON FUNCTION ops.guard_order_attempt_subject() IS
 'Internal trigger only, order-attempt-v1 §2: no terminal subject index or subject reassignment; admission authenticates and fences the complete subject set.';
CREATE TRIGGER order_attempt_subject_pending BEFORE INSERT OR UPDATE ON ops.order_attempt_subjects
 FOR EACH ROW EXECUTE FUNCTION ops.guard_order_attempt_subject();
