-- Purpose: OC-02 recent re-authentication core (contracts/merchant-password-auth-v1.md §16, ADR-20261026 T21R-01):
--   identity.sessions.authenticated_at (backfilled, then NOT NULL DEFAULT clock_timestamp()), the fail-closed check
--   identity.require_recent_auth, the password + emailed-code re-auth definers (purpose 'reauth' challenges bound to
--   the calling session hash) and the OIDC re-auth definers (login_flows bound to the calling session, principal
--   match in SQL). Every issue path (0004 issue_merchant_session, 0070 complete_email_challenge) now writes
--   authenticated_at = clock_timestamp(); refresh never moves it.
-- Depends on: 0001 (identity.sessions), 0004 (login_flows, external_identities, session_events, the identity roles),
--   0070 (email_challenges, password_credentials, auth_events, complete_email_challenge).
-- Used by: internal/identity (reauth.go, oidc_reauth.go, service.go) through the commerce_identity login; the guarded
--   business definers call identity.require_recent_auth in their own transaction (OC-03 grants them EXECUTE).
-- Non-goals: no guard on any business definer (OC-03), no HTTP mapping (OC-15). Re-runnable: every DDL is
--   IF [NOT] EXISTS / CREATE OR REPLACE, so tests/identity RA03 can replay it over the pre-0201 shape.

-- Session freshness. Added nullable, backfilled from the session's session.issued event, -infinity when there is
-- none (that session must re-authenticate), and only then NOT NULL + DEFAULT, so no old session is marked fresh.
ALTER TABLE identity.sessions ADD COLUMN IF NOT EXISTS authenticated_at timestamptz;
UPDATE identity.sessions s SET authenticated_at = e.created_at
  FROM identity.session_events e
 WHERE e.session_id = s.id AND e.action = 'session.issued' AND s.authenticated_at IS NULL;
UPDATE identity.sessions SET authenticated_at = '-infinity' WHERE authenticated_at IS NULL;
ALTER TABLE identity.sessions ALTER COLUMN authenticated_at SET NOT NULL;
-- The default only covers direct test fixtures; every definer that issues a session writes the column itself.
ALTER TABLE identity.sessions ALTER COLUMN authenticated_at SET DEFAULT clock_timestamp();
GRANT UPDATE (authenticated_at) ON identity.sessions TO commerce_identity_writer;
COMMENT ON COLUMN identity.sessions.authenticated_at IS 'Last proof of the credential (§16): clock_timestamp() at issue, moved only by identity.complete_reauth_challenge / identity.complete_oidc_reauth, never by refresh. -infinity = issued before 0201 without a session.issued event (must re-authenticate). Checked by identity.require_recent_auth.';

-- OIDC re-auth flows carry the hash of the calling session; ordinary login flows keep it NULL.
ALTER TABLE identity.login_flows ADD COLUMN IF NOT EXISTS session_hash bytea;
ALTER TABLE identity.login_flows DROP CONSTRAINT IF EXISTS login_flows_session_hash_check;
ALTER TABLE identity.login_flows ADD CONSTRAINT login_flows_session_hash_check CHECK (session_hash IS NULL OR octet_length(session_hash) = 32);
COMMENT ON COLUMN identity.login_flows.session_hash IS 'OIDC re-auth only (§16): sha256 token hash of the session that started the flow; NULL for an ordinary login flow. A flow with it set never issues a session.';

-- Re-auth challenges: purpose 'reauth', bound to the principal, the password version and the calling session hash.
ALTER TABLE identity.email_challenges ADD COLUMN IF NOT EXISTS session_hash bytea;
ALTER TABLE identity.email_challenges DROP CONSTRAINT IF EXISTS email_challenges_purpose_check;
ALTER TABLE identity.email_challenges ADD CONSTRAINT email_challenges_purpose_check CHECK (purpose IN ('signup','login','reset','reauth'));
ALTER TABLE identity.email_challenges DROP CONSTRAINT IF EXISTS email_challenges_session_hash_check;
ALTER TABLE identity.email_challenges ADD CONSTRAINT email_challenges_session_hash_check CHECK (session_hash IS NULL OR octet_length(session_hash) = 32);
ALTER TABLE identity.email_challenges DROP CONSTRAINT IF EXISTS email_challenges_shape;
ALTER TABLE identity.email_challenges ADD CONSTRAINT email_challenges_shape CHECK (
    (purpose = 'signup' AND principal_id IS NULL AND password_version IS NULL AND session_hash IS NULL
        AND (pending_password_hash IS NOT NULL OR consumed_at IS NOT NULL))
    OR (purpose = 'login' AND principal_id IS NOT NULL AND password_version IS NOT NULL AND pending_password_hash IS NULL AND session_hash IS NULL)
    OR (purpose = 'reset' AND principal_id IS NOT NULL AND password_version IS NULL AND pending_password_hash IS NULL AND session_hash IS NULL)
    OR (purpose = 'reauth' AND principal_id IS NOT NULL AND password_version IS NOT NULL AND pending_password_hash IS NULL AND session_hash IS NOT NULL));
CREATE INDEX IF NOT EXISTS email_challenges_reauth_open ON identity.email_challenges(session_hash) WHERE purpose = 'reauth' AND consumed_at IS NULL;
CREATE INDEX IF NOT EXISTS email_challenges_reauth_budget ON identity.email_challenges(principal_id, created_at) WHERE purpose = 'reauth';
COMMENT ON COLUMN identity.email_challenges.session_hash IS 'reauth only (§16): sha256 token hash of the session the code may refresh; the code HMAC is keyed on it, so a code issued to session A cannot refresh session B. NULL for signup/login/reset.';
COMMENT ON COLUMN identity.email_challenges.purpose IS 'signup | login | reset | reauth; the purpose shape CHECK ties it to principal/version/pending hash/session hash.';

ALTER TABLE identity.auth_events DROP CONSTRAINT IF EXISTS auth_events_action_check;
ALTER TABLE identity.auth_events ADD CONSTRAINT auth_events_action_check CHECK (action IN ('signup.requested','signup.exists_notified','signup.verified',
    'login.password_failed','login.password_ok','login.disabled','login.verified','reset.requested',
    'reset.completed','code.failed','code.exhausted','throttled','mail.sent','mail.failed','mail.unknown',
    'password.breach_check_unavailable','operator.unlocked','reauth.requested','reauth.verified','reauth.refused'));

-- ---------------------------------------------------------------------------------------------
-- Issue paths: same bodies as 0004 / 0070, plus authenticated_at = clock_timestamp() in the session INSERT.
-- CREATE OR REPLACE keeps owner and ACL.
-- ---------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION identity.issue_merchant_session(p_issuer text,p_subject text,p_hash bytea,p_ttl bigint)
RETURNS TABLE(principal_id uuid,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_principal uuid; v_active boolean; v_session uuid; v_expiry timestamptz;
BEGIN
    IF p_ttl IS NULL OR p_ttl<300 OR p_ttl>86400 OR p_issuer IS NULL OR length(p_issuer) NOT BETWEEN 1 AND 2048
       OR p_subject IS NULL OR length(p_subject) NOT BETWEEN 1 AND 255 THEN RAISE EXCEPTION 'invalid identity request' USING ERRCODE='PT400'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(json_build_array(p_issuer,p_subject)::text,0));
    SELECT p.id,p.active INTO v_principal,v_active FROM identity.external_identities e
      JOIN identity.principals p ON p.id=e.principal_id WHERE e.issuer=p_issuer AND e.subject=p_subject FOR UPDATE OF p;
    IF NOT FOUND THEN
        INSERT INTO identity.principals(id) VALUES(gen_random_uuid()) RETURNING id INTO v_principal;
        INSERT INTO identity.external_identities(issuer,subject,principal_id) VALUES(p_issuer,p_subject,v_principal);
    ELSIF NOT v_active THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    INSERT INTO identity.sessions(id,token_hash,principal_id,audience,expires_at,authenticated_at)
      VALUES(gen_random_uuid(),p_hash,v_principal,'merchant',clock_timestamp()+p_ttl*interval '1 second',clock_timestamp()) RETURNING id,identity.sessions.expires_at INTO v_session,v_expiry;
    INSERT INTO identity.session_events(principal_id,session_id,action) VALUES(v_principal,v_session,'session.issued');
    RETURN QUERY SELECT v_principal,v_expiry;
END $$;

CREATE OR REPLACE FUNCTION identity.complete_email_challenge(p_binding bytea, p_purpose text, p_code bytea, p_new_hash text, p_session bytea, p_ttl bigint)
RETURNS TABLE(outcome text, expires_at timestamptz) LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE
    v_id uuid; v_email text; v_principal uuid; v_ch identity.email_challenges%ROWTYPE;
    v_active boolean; v_cred identity.password_credentials%ROWTYPE; v_cred_found boolean := false;
    v_same bigint; v_all bigint; v_attempts smallint; v_session uuid; v_expiry timestamptz;
BEGIN
    IF p_binding IS NULL OR octet_length(p_binding) <> 32 OR p_purpose IS NULL OR p_purpose NOT IN ('signup','login','reset')
       OR p_code IS NULL OR octet_length(p_code) <> 32 OR p_session IS NULL OR octet_length(p_session) <> 32
       OR p_ttl IS NULL OR p_ttl NOT BETWEEN 300 AND 86400
       OR (p_purpose = 'reset' AND p_new_hash IS NULL) OR (p_purpose <> 'reset' AND p_new_hash IS NOT NULL) THEN
        RAISE EXCEPTION 'invalid challenge request' USING ERRCODE = 'PT400';
    END IF;
    -- Unlocked lookup only to learn the email/principal for the lock order; everything is rechecked after locking.
    SELECT c.id, c.email, c.principal_id INTO v_id, v_email, v_principal
      FROM identity.email_challenges c WHERE c.binding_hash = p_binding AND c.purpose = p_purpose;
    IF NOT FOUND THEN RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('lc:pw-email:' || v_email, 0));
    IF v_principal IS NOT NULL THEN
        SELECT p.active INTO v_active FROM identity.principals p WHERE p.id = v_principal FOR UPDATE;
        SELECT * INTO v_cred FROM identity.password_credentials c WHERE c.principal_id = v_principal FOR UPDATE;
        v_cred_found := FOUND;
    END IF;
    SELECT * INTO v_ch FROM identity.email_challenges c WHERE c.id = v_id FOR UPDATE;
    IF NOT FOUND OR v_ch.consumed_at IS NOT NULL OR v_ch.expires_at <= clock_timestamp() THEN
        RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN;
    END IF;
    IF p_purpose = 'reset' THEN
        -- PD5: budget per (principal, source ip bucket) 10 and per principal 50 over 24 h. The principal
        -- row lock above serializes concurrent completes; over budget = INVALID with no compare and no attempts change.
        SELECT COALESCE(sum(c.attempts) FILTER (WHERE c.ip_hmac = v_ch.ip_hmac), 0), COALESCE(sum(c.attempts), 0)
          INTO v_same, v_all
          FROM identity.email_challenges c
         WHERE c.principal_id = v_principal AND c.purpose = 'reset' AND c.created_at > clock_timestamp() - interval '24 hours';
        IF v_same >= 10 OR v_all >= 50 THEN RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN; END IF;
    END IF;
    -- Both sides are HMACs of server-side secrets, so plain <> leaks nothing useful (§4.3 step 3).
    IF v_ch.code_hmac <> p_code THEN
        v_attempts := v_ch.attempts + 1;
        IF v_attempts >= 5 THEN
            UPDATE identity.email_challenges c SET attempts = v_attempts, consumed_at = clock_timestamp(),
                   consumed_reason = 'exhausted', pending_password_hash = NULL WHERE c.id = v_id;
            INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'code.exhausted');
        ELSE
            UPDATE identity.email_challenges c SET attempts = v_attempts WHERE c.id = v_id;
        END IF;
        INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'code.failed');
        RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz;
        RETURN;
    END IF;

    IF p_purpose = 'signup' THEN
        UPDATE identity.email_challenges c SET consumed_at = clock_timestamp(), consumed_reason = 'verified', pending_password_hash = NULL WHERE c.id = v_id;
        IF EXISTS (SELECT 1 FROM identity.password_credentials c WHERE c.email = v_email) THEN
            RETURN QUERY SELECT 'EXISTS'::text, NULL::timestamptz; RETURN;  -- lost a race with another sign-up
        END IF;
        INSERT INTO identity.principals(id) VALUES (gen_random_uuid()) RETURNING id INTO v_principal;
        INSERT INTO identity.password_credentials(principal_id, email, password_hash, email_verified_at)
        VALUES (v_principal, v_email, v_ch.pending_password_hash, clock_timestamp());
        INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'signup.verified');
    ELSIF p_purpose = 'login' THEN
        IF NOT v_cred_found OR NOT v_active OR v_cred.disabled_at IS NOT NULL OR v_cred.password_version <> v_ch.password_version THEN
            RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN;
        END IF;
        UPDATE identity.email_challenges c SET consumed_at = clock_timestamp(), consumed_reason = 'verified' WHERE c.id = v_id;
        UPDATE identity.password_credentials c SET failed_count = 0 WHERE c.principal_id = v_principal;
        INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'login.verified');
    ELSE
        IF NOT v_cred_found OR NOT v_active THEN
            RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN;
        END IF;
        UPDATE identity.email_challenges c SET consumed_at = clock_timestamp(), consumed_reason = 'verified' WHERE c.id = v_id;
        -- PD10: new hash, version+1, counter reset, re-enabled.
        UPDATE identity.password_credentials c SET password_hash = p_new_hash, password_version = c.password_version + 1,
               failed_count = 0, disabled_at = NULL, password_changed_at = clock_timestamp()
         WHERE c.principal_id = v_principal;
        -- Revoke every unrevoked merchant session of the principal (other audiences untouched), with events.
        WITH revoked AS (UPDATE identity.sessions s SET revoked_at = clock_timestamp()
                          WHERE s.principal_id = v_principal AND s.audience = 'merchant' AND s.revoked_at IS NULL RETURNING s.id)
        INSERT INTO identity.session_events(principal_id, session_id, action) SELECT v_principal, r.id, 'session.revoked' FROM revoked r;
        -- Older open login challenges were bound to the old password version; retire them.
        UPDATE identity.email_challenges c SET consumed_at = clock_timestamp(), consumed_reason = 'superseded'
         WHERE c.principal_id = v_principal AND c.purpose = 'login' AND c.consumed_at IS NULL;
        INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'reset.completed');
    END IF;

    -- PD8: a session is issued only here, in the definer that consumed a correct code (§16: authenticated now).
    INSERT INTO identity.sessions(id, token_hash, principal_id, audience, expires_at, authenticated_at)
    VALUES (gen_random_uuid(), p_session, v_principal, 'merchant', clock_timestamp() + p_ttl * interval '1 second', clock_timestamp())
    RETURNING identity.sessions.id, identity.sessions.expires_at INTO v_session, v_expiry;
    INSERT INTO identity.session_events(principal_id, session_id, action) VALUES (v_principal, v_session, 'session.issued');
    RETURN QUERY SELECT 'SESSION'::text, v_expiry;
END $$;

-- An ordinary login flow never consumes a re-auth flow (that would issue a new session for it).
CREATE OR REPLACE FUNCTION identity.consume_login_flow(p_state bytea,p_binding bytea,p_provider text)
RETURNS TABLE(nonce text,verifier text) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
    DELETE FROM identity.login_flows WHERE state_hash=p_state AND binding_hash=p_binding
      AND provider_key=p_provider AND expires_at>clock_timestamp() AND session_hash IS NULL RETURNING nonce,verifier
$$;

-- ---------------------------------------------------------------------------------------------
-- §16 definers. SECURITY DEFINER, owner commerce_identity_writer, search_path = pg_catalog, PUBLIC revoked,
-- EXECUTE for commerce_identity only. Lock order as 0070: email advisory lock -> principal -> credential ->
-- challenge -> sessions.
-- ---------------------------------------------------------------------------------------------

-- The check every guarded definer runs in its own transaction. Fails closed: a NULL, unknown, revoked or expired
-- hash, an inactive principal, a disabled password credential (PD12) or a stale authenticated_at all raise.
CREATE OR REPLACE FUNCTION identity.require_recent_auth(p_hash bytea, p_window interval)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_at timestamptz;
BEGIN
    IF p_window IS NULL OR p_window < interval '1 minute' OR p_window > interval '60 minutes' THEN
        RAISE EXCEPTION 'invalid reauth window' USING ERRCODE = 'PT400';
    END IF;
    SELECT s.authenticated_at INTO v_at
      FROM identity.sessions s JOIN identity.principals p ON p.id = s.principal_id
     WHERE p_hash IS NOT NULL AND s.token_hash = p_hash AND s.audience = 'merchant'
       AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp() AND p.active
       AND NOT EXISTS (SELECT 1 FROM identity.password_credentials c WHERE c.principal_id = s.principal_id AND c.disabled_at IS NOT NULL);
    -- Compared as v_at < now() - window (not now() - v_at > window): -infinity minus would overflow the interval.
    IF NOT FOUND OR v_at IS NULL OR v_at < now() - p_window THEN
        RAISE EXCEPTION 'reauth_required' USING ERRCODE = 'PT403';
    END IF;
END $$;

-- Start step material: the calling live merchant session's principal and, when it has one, its password
-- credential. Zero rows when the session is not live; NULL email for an OIDC-only principal.
CREATE OR REPLACE FUNCTION identity.reauth_material(p_session bytea)
RETURNS TABLE(principal_id uuid, email text, password_hash text, password_version bigint, disabled boolean)
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT s.principal_id, c.email, c.password_hash, c.password_version, c.disabled_at IS NOT NULL
      FROM identity.sessions s JOIN identity.principals p ON p.id = s.principal_id
      LEFT JOIN identity.password_credentials c ON c.principal_id = s.principal_id
     WHERE p_session IS NOT NULL AND s.token_hash = p_session AND s.audience = 'merchant'
       AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp() AND p.active
$$;

-- After Go verified the password: supersede this session's open reauth challenges and insert one bound to it.
CREATE OR REPLACE FUNCTION identity.start_reauth_challenge(p_id uuid, p_session bytea, p_version bigint, p_binding bytea, p_code bytea, p_locale text, p_ip bytea)
RETURNS timestamptz LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE v_principal uuid; v_email text; v_active boolean; v_version bigint; v_disabled timestamptz; v_expiry timestamptz;
BEGIN
    IF p_id IS NULL OR p_session IS NULL OR octet_length(p_session) <> 32 OR p_version IS NULL
       OR p_binding IS NULL OR octet_length(p_binding) <> 32 OR p_code IS NULL OR octet_length(p_code) <> 32
       OR p_ip IS NULL OR octet_length(p_ip) <> 32 OR p_locale IS NULL OR p_locale NOT IN ('zh-CN','zh-TW','en') THEN
        RAISE EXCEPTION 'invalid challenge request' USING ERRCODE = 'PT400';
    END IF;
    SELECT s.principal_id INTO v_principal FROM identity.sessions s WHERE s.token_hash = p_session AND s.audience = 'merchant';
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    SELECT c.email INTO v_email FROM identity.password_credentials c WHERE c.principal_id = v_principal;
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('lc:pw-email:' || v_email, 0));
    SELECT p.active INTO v_active FROM identity.principals p WHERE p.id = v_principal FOR UPDATE;
    SELECT c.password_version, c.disabled_at INTO v_version, v_disabled
      FROM identity.password_credentials c WHERE c.principal_id = v_principal FOR UPDATE;
    IF NOT FOUND OR NOT v_active OR v_disabled IS NOT NULL OR v_version <> p_version
       OR NOT EXISTS (SELECT 1 FROM identity.sessions s WHERE s.token_hash = p_session AND s.principal_id = v_principal
                        AND s.audience = 'merchant' AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp()) THEN
        RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401';
    END IF;
    UPDATE identity.email_challenges c SET consumed_at = clock_timestamp(), consumed_reason = 'superseded'
     WHERE c.session_hash = p_session AND c.purpose = 'reauth' AND c.consumed_at IS NULL;
    INSERT INTO identity.email_challenges(id, purpose, email, locale, principal_id, password_version, binding_hash, code_hmac, ip_hmac, session_hash, expires_at)
    VALUES (p_id, 'reauth', v_email, p_locale, v_principal, p_version, p_binding, p_code, p_ip, p_session, clock_timestamp() + interval '10 minutes')
    RETURNING identity.email_challenges.expires_at INTO v_expiry;
    INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, p_ip, p_id, 'reauth.requested');
    RETURN v_expiry;
END $$;

-- Complete step: the open reauth challenge of the CALLING session only; updates only that session's
-- authenticated_at; inserts no session and no session.issued event. INVALID is returned, not raised, so a
-- wrong-code attempt counter commits.
CREATE OR REPLACE FUNCTION identity.complete_reauth_challenge(p_session bytea, p_code bytea)
RETURNS TABLE(outcome text, authenticated_at timestamptz) LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE
    v_id uuid; v_email text; v_principal uuid; v_ch identity.email_challenges%ROWTYPE; v_active boolean;
    v_cred identity.password_credentials%ROWTYPE; v_cred_found boolean; v_same bigint; v_all bigint;
    v_attempts smallint; v_session uuid; v_at timestamptz;
BEGIN
    IF p_session IS NULL OR octet_length(p_session) <> 32 OR p_code IS NULL OR octet_length(p_code) <> 32 THEN
        RAISE EXCEPTION 'invalid challenge request' USING ERRCODE = 'PT400';
    END IF;
    SELECT c.id, c.email, c.principal_id INTO v_id, v_email, v_principal FROM identity.email_challenges c
     WHERE c.session_hash = p_session AND c.purpose = 'reauth' AND c.consumed_at IS NULL ORDER BY c.created_at DESC, c.id LIMIT 1;
    IF NOT FOUND THEN RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('lc:pw-email:' || v_email, 0));
    SELECT p.active INTO v_active FROM identity.principals p WHERE p.id = v_principal FOR UPDATE;
    SELECT * INTO v_cred FROM identity.password_credentials c WHERE c.principal_id = v_principal FOR UPDATE;
    v_cred_found := FOUND;
    SELECT * INTO v_ch FROM identity.email_challenges c WHERE c.id = v_id FOR UPDATE;
    IF NOT FOUND OR v_ch.consumed_at IS NOT NULL OR v_ch.expires_at <= clock_timestamp() THEN
        RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN;
    END IF;
    -- PD5 budget applies to reauth challenges: >= 10 per ip bucket or >= 50 in total in 24 h -> refused, no compare.
    SELECT COALESCE(sum(c.attempts) FILTER (WHERE c.ip_hmac = v_ch.ip_hmac), 0), COALESCE(sum(c.attempts), 0) INTO v_same, v_all
      FROM identity.email_challenges c
     WHERE c.principal_id = v_principal AND c.purpose = 'reauth' AND c.created_at > clock_timestamp() - interval '24 hours';
    IF v_same >= 10 OR v_all >= 50 THEN RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN; END IF;
    IF v_ch.code_hmac <> p_code THEN
        v_attempts := v_ch.attempts + 1;
        IF v_attempts >= 5 THEN
            UPDATE identity.email_challenges c SET attempts = v_attempts, consumed_at = clock_timestamp(), consumed_reason = 'exhausted' WHERE c.id = v_id;
            INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'code.exhausted');
        ELSE
            UPDATE identity.email_challenges c SET attempts = v_attempts WHERE c.id = v_id;
        END IF;
        INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'code.failed');
        RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN;
    END IF;
    IF NOT v_cred_found OR NOT v_active OR v_cred.disabled_at IS NOT NULL OR v_cred.password_version <> v_ch.password_version THEN
        RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN;
    END IF;
    SELECT s.id INTO v_session FROM identity.sessions s
     WHERE s.token_hash = p_session AND s.principal_id = v_principal AND s.audience = 'merchant'
       AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp() FOR UPDATE;
    IF NOT FOUND THEN RETURN QUERY SELECT 'INVALID'::text, NULL::timestamptz; RETURN; END IF;
    UPDATE identity.email_challenges c SET consumed_at = clock_timestamp(), consumed_reason = 'verified' WHERE c.id = v_id;
    UPDATE identity.password_credentials c SET failed_count = 0 WHERE c.principal_id = v_principal;
    UPDATE identity.sessions s SET authenticated_at = clock_timestamp() WHERE s.id = v_session RETURNING s.authenticated_at INTO v_at;
    INSERT INTO identity.auth_events(principal_id, ip_hmac, challenge_id, action) VALUES (v_principal, v_ch.ip_hmac, v_id, 'reauth.verified');
    RETURN QUERY SELECT 'VERIFIED'::text, v_at;
END $$;

-- OIDC re-auth start: a session-authenticated flow; the calling session hash is stored in the flow binding.
CREATE OR REPLACE FUNCTION identity.start_oidc_reauth_flow(p_state bytea, p_binding bytea, p_provider text, p_nonce text, p_verifier text, p_session bytea)
RETURNS timestamptz LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_expiry timestamptz;
BEGIN
    IF p_session IS NULL OR octet_length(p_session) <> 32 THEN RAISE EXCEPTION 'invalid identity request' USING ERRCODE = 'PT400'; END IF;
    PERFORM 1 FROM identity.sessions s JOIN identity.principals p ON p.id = s.principal_id
     WHERE s.token_hash = p_session AND s.audience = 'merchant' AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp() AND p.active;
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    INSERT INTO identity.login_flows(state_hash, binding_hash, provider_key, nonce, verifier, expires_at, session_hash)
    VALUES (p_state, p_binding, p_provider, p_nonce, p_verifier, clock_timestamp() + interval '5 minutes', p_session)
    RETURNING identity.login_flows.expires_at INTO v_expiry;
    RETURN v_expiry;
END $$;

-- The callback's one-use flow consumption for both kinds: session_hash NULL = ordinary login, set = re-auth.
CREATE OR REPLACE FUNCTION identity.consume_login_flow_with_session(p_state bytea, p_binding bytea, p_provider text)
RETURNS TABLE(nonce text, verifier text, session_hash bytea) LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    DELETE FROM identity.login_flows WHERE state_hash = p_state AND binding_hash = p_binding
      AND provider_key = p_provider AND expires_at > clock_timestamp() RETURNING nonce, verifier, session_hash
$$;

-- OIDC re-auth complete: the principal resolved from external_identities(issuer, subject) must equal the calling
-- session's principal and auth_time must be within 10 minutes; otherwise REFUSED with authenticated_at unchanged.
-- Never issues a session or a session.issued event. Success sets authenticated_at to the IdP auth_time (never later
-- than now), the actual moment the credential was proven.
CREATE OR REPLACE FUNCTION identity.complete_oidc_reauth(p_session_hash bytea, p_issuer text, p_subject text, p_auth_time timestamptz)
RETURNS TABLE(outcome text, authenticated_at timestamptz) LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE v_principal uuid; v_session uuid; v_other uuid; v_at timestamptz;
BEGIN
    IF p_session_hash IS NULL OR octet_length(p_session_hash) <> 32 OR p_issuer IS NULL OR length(p_issuer) NOT BETWEEN 1 AND 2048
       OR p_subject IS NULL OR length(p_subject) NOT BETWEEN 1 AND 255 THEN
        RAISE EXCEPTION 'invalid identity request' USING ERRCODE = 'PT400';
    END IF;
    SELECT s.principal_id INTO v_principal FROM identity.sessions s WHERE s.token_hash = p_session_hash AND s.audience = 'merchant';
    IF NOT FOUND THEN RETURN QUERY SELECT 'UNAUTHORIZED'::text, NULL::timestamptz; RETURN; END IF;
    PERFORM 1 FROM identity.principals p WHERE p.id = v_principal AND p.active FOR UPDATE;
    IF NOT FOUND THEN RETURN QUERY SELECT 'UNAUTHORIZED'::text, NULL::timestamptz; RETURN; END IF;
    SELECT s.id INTO v_session FROM identity.sessions s
     WHERE s.token_hash = p_session_hash AND s.principal_id = v_principal AND s.audience = 'merchant'
       AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp() FOR UPDATE;
    IF NOT FOUND THEN RETURN QUERY SELECT 'UNAUTHORIZED'::text, NULL::timestamptz; RETURN; END IF;
    SELECT e.principal_id INTO v_other FROM identity.external_identities e WHERE e.issuer = p_issuer AND e.subject = p_subject;
    IF v_other IS DISTINCT FROM v_principal OR p_auth_time IS NULL
       OR p_auth_time < clock_timestamp() - interval '10 minutes' OR p_auth_time > clock_timestamp() + interval '1 minute' THEN
        INSERT INTO identity.auth_events(principal_id, action) VALUES (v_principal, 'reauth.refused');
        RETURN QUERY SELECT 'REFUSED'::text, NULL::timestamptz; RETURN;
    END IF;
    UPDATE identity.sessions s SET authenticated_at = LEAST(p_auth_time, clock_timestamp()) WHERE s.id = v_session
    RETURNING s.authenticated_at INTO v_at;
    INSERT INTO identity.auth_events(principal_id, action) VALUES (v_principal, 'reauth.verified');
    RETURN QUERY SELECT 'REAUTHENTICATED'::text, v_at;
END $$;

ALTER FUNCTION identity.require_recent_auth(bytea,interval) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.reauth_material(bytea) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.start_reauth_challenge(uuid,bytea,bigint,bytea,bytea,text,bytea) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.complete_reauth_challenge(bytea,bytea) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.start_oidc_reauth_flow(bytea,bytea,text,text,text,bytea) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.consume_login_flow_with_session(bytea,bytea,text) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.complete_oidc_reauth(bytea,text,text,timestamptz) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION identity.require_recent_auth(bytea,interval), identity.reauth_material(bytea),
    identity.start_reauth_challenge(uuid,bytea,bigint,bytea,bytea,text,bytea), identity.complete_reauth_challenge(bytea,bytea),
    identity.start_oidc_reauth_flow(bytea,bytea,text,text,text,bytea), identity.consume_login_flow_with_session(bytea,bytea,text),
    identity.complete_oidc_reauth(bytea,text,text,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.require_recent_auth(bytea,interval), identity.reauth_material(bytea),
    identity.start_reauth_challenge(uuid,bytea,bigint,bytea,bytea,text,bytea), identity.complete_reauth_challenge(bytea,bytea),
    identity.start_oidc_reauth_flow(bytea,bytea,text,text,text,bytea), identity.consume_login_flow_with_session(bytea,bytea,text),
    identity.complete_oidc_reauth(bytea,text,text,timestamptz) TO commerce_identity;

COMMENT ON FUNCTION identity.require_recent_auth(bytea,interval) IS 'Owner: internal/identity (§16). Raises PT403 reauth_required unless the hash names a live merchant session of an active principal without a disabled password credential and now() - authenticated_at <= p_window (1-60 minutes, else PT400). Called inside each guarded definer in the write transaction; EXECUTE commerce_identity, plus the guarded owners (OC-03).';
COMMENT ON FUNCTION identity.reauth_material(bytea) IS 'Owner: internal/identity (reauth.go). Role: commerce_identity only. The calling live session principal and its password credential (NULLs for an OIDC-only principal); zero rows when the session is not live.';
COMMENT ON FUNCTION identity.start_reauth_challenge(uuid,bytea,bigint,bytea,bytea,text,bytea) IS 'Owner: internal/identity (reauth.go). Role: commerce_identity only. After Go verified the password: requires the live session, credential version and an enabled credential, supersedes the session''s open reauth challenges and inserts one bound to the session hash. PT401 otherwise.';
COMMENT ON FUNCTION identity.complete_reauth_challenge(bytea,bytea) IS 'Owner: internal/identity (reauth.go). Role: commerce_identity only. Verifies the code of the calling session''s open reauth challenge (PD5 budget, 5 attempts), then updates only that session''s authenticated_at. VERIFIED or INVALID; never inserts a session or a session.issued event.';
COMMENT ON FUNCTION identity.start_oidc_reauth_flow(bytea,bytea,text,text,text,bytea) IS 'Owner: internal/identity (oidc_reauth.go). Role: commerce_identity only. Session-authenticated OIDC re-auth start: inserts a 5-minute login flow carrying the calling session hash. PT401 when the session is not live.';
COMMENT ON FUNCTION identity.consume_login_flow_with_session(bytea,bytea,text) IS 'Owner: internal/identity (service.go). Role: commerce_identity only. One-use consumption of a login flow of either kind; session_hash tells the callback whether it is a re-auth (complete_oidc_reauth) or a login (issue_merchant_session).';
COMMENT ON FUNCTION identity.complete_oidc_reauth(bytea,text,text,timestamptz) IS 'Owner: internal/identity (oidc_reauth.go). Role: commerce_identity only. REAUTHENTICATED only when external_identities(issuer, subject) resolves to the calling session''s principal and auth_time is within 10 minutes; else REFUSED (authenticated_at unchanged) or UNAUTHORIZED (session not live). Never issues a session or a session.issued event.';
