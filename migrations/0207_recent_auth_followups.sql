-- Purpose: IDENT-P2 follow-ups for the recent re-authentication guards (#215 OC-02, #228 OC-03).
--   (1) #228 item 6: a session whose authenticated_at is infinity stays infinite through the previous
--       GREATEST(s.authenticated_at, LEAST(p_auth_time, clock_timestamp())). Cap the result at clock_timestamp() so a
--       successful OIDC re-auth always leaves a finite value.
-- Depends on: 0201, 0203, post_river/0028 (the guards under test).
-- Note (#228 item 3): 0203 is checksum-frozen, so its header wording cannot be edited; the staff trio actually takes
--   the staff_owner_ctx read lock before the guard. output/oc-03/DEFINERS.md records that order as the source of truth.
-- Used by: tests/identity (OG04 ordering, OG07 infinity cap), the guarded definers.
-- Migration rules: forward-only. The function is patched IN PLACE with pg_get_functiondef + exactly-one-anchor
--   replace; CREATE OR REPLACE keeps owner, ACL and signature. No data is changed.
-- Status: REAL_PG only.

DO $do$
DECLARE v_fn text; v_old text; v_new text; v_hits int;
BEGIN
    -- (1) complete_oidc_reauth: cap an infinite old value at the database clock.
    v_fn := pg_get_functiondef('identity.complete_oidc_reauth(bytea,text,text,timestamptz)'::regprocedure);
    v_old := $n$SET authenticated_at = GREATEST(s.authenticated_at, LEAST(p_auth_time, clock_timestamp()))$n$;
    v_new := $n$SET authenticated_at = LEAST(GREATEST(s.authenticated_at, LEAST(p_auth_time, clock_timestamp())), clock_timestamp())$n$;
    v_hits := (length(v_fn)-length(replace(v_fn,v_old,'')))/length(v_old);
    IF v_hits <> 1 THEN
        RAISE EXCEPTION 'identity.complete_oidc_reauth has an unexpected shape for the 0207 patch: %', v_hits;
    END IF;
    EXECUTE replace(v_fn, v_old, v_new);
END $do$;

COMMENT ON FUNCTION identity.complete_oidc_reauth(bytea,text,text,timestamptz) IS 'Owner: internal/identity (oidc_reauth.go). Role: commerce_identity only. REAUTHENTICATED only when external_identities(issuer, subject) resolves to the calling session''s principal and auth_time is within 10 minutes; the result is GREATEST(old authenticated_at, LEAST(auth_time, now)) capped at now, so a prior infinity never survives. Else REFUSED (authenticated_at unchanged) or UNAUTHORIZED (session not live). Never issues a session or a session.issued event.';
