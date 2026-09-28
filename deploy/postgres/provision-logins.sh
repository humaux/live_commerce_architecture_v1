#!/usr/bin/env bash
# File: deploy/postgres/provision-logins.sh
# Purpose: idempotently create/alter the service LOGIN roles listed as `core` in logins.tsv,
#   grant each exactly one business authority with the tested grant shape, then VERIFY
#   (never auto-repair) the membership matrix, authenticate every login over TCP like the
#   services do, and check the DB readiness gates (deploy-design §9.4).
# Runs as/in: one-shot service "provision-logins" (postgres image, UID 999, bash + psql),
#   after "migrate" succeeded. Connects as the bootstrap superuser over the unix socket
#   (/var/run/postgresql, shared pgsocket volume); pg_hba forbids the superuser over TCP.
# Reads env: LC_PG_HOST (TCP host for the login check, "postgres"), PGAPPNAME,
#   LC_REQUIRE_MEDIA_GATE (0|1, default 0: live.media_plan_ready() informational, see step 6).
#   Optional test overrides: LC_LOGINS_TSV, LC_SECRETS_MOUNT.
# Reads secrets: /run/secrets/pg_superuser_password (superuser session) and
#   /run/secrets/pw_<login> for each core login (password to set and to test). Passwords are
#   read by psql itself (\set pw `cat file`) or passed to a child psql via PGPASSWORD only;
#   they never appear in argv, stdout or the server log (session log_* settings below).
# Used by: deploy/compose.yml (service provision-logins), deploy/scripts/deploy.sh (first,
#   upgrade), smoke S13; docs/runbooks/incident.md §DB 角色 for drift repair.
# Depends on: migrations applied (authority roles + readiness functions exist),
#   deploy/postgres/logins.tsv mounted at /provision/logins.tsv.
# Status: DESIGN; verified by smoke S13 (twice, idempotent) and S27 (no password in logs).
# Change rules: grant shapes must match the repo tests cited in logins.tsv and the checks in
#   internal/platform/platform.go validatePoolAuthority. On drift this script exits 1 and
#   names login + problem; the fix is a reviewed manual REVOKE, never an automatic one.
set -Eeuo pipefail
umask 077

TSV=${LC_LOGINS_TSV:-/provision/logins.tsv}
SECRETS=${LC_SECRETS_MOUNT:-/run/secrets}
PG_SOCKET=/var/run/postgresql
DB=live_commerce
PG_HOST=${LC_PG_HOST:-postgres}
export PGAPPNAME=${PGAPPNAME:-lc-provision-logins} PGCONNECT_TIMEOUT=10

log() { printf '%s provision-logins %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2; }
die() {
  log "FAIL $*"
  exit 1
}

[[ -r "$TSV" ]] || die "logins.tsv not readable"
[[ -r "$SECRETS/pg_superuser_password" ]] || die "missing secret pg_superuser_password"

# Superuser psql over the socket; SQL arrives on stdin, the password only via the environment
# of this one child process.
su_psql() {
  PGPASSWORD="$(<"$SECRETS/pg_superuser_password")" \
    psql -X -q -At -F '|' -v ON_ERROR_STOP=1 -h "$PG_SOCKET" -U postgres -d "$DB"
}

# ---- 1. parse logins.tsv (core rows only) ----------------------------------------------------
logins=() authorities=() grants=()
while IFS=$'\t' read -r login authority grant set _consumer _proof; do
  [[ -z "$login" || "$login" == \#* ]] && continue
  [[ "$set" == core ]] || continue
  [[ "$login" =~ ^lc_[a-z_]{1,40}$ ]] || die "invalid login name in logins.tsv"
  [[ "$authority" =~ ^commerce_[a-z_]{1,50}$ ]] || die "invalid authority for $login"
  [[ "$grant" == in_role || "$grant" == inherit_noset ]] || die "invalid grant shape for $login"
  [[ -r "$SECRETS/pw_$login" ]] || die "missing secret pw_$login"
  logins+=("$login") authorities+=("$authority") grants+=("$grant")
done <"$TSV"
((${#logins[@]} > 0)) || die "no core logins in logins.tsv"
log "core logins: ${#logins[@]}"

# ---- 2. migrations must already be applied ---------------------------------------------------
ledger="$(su_psql <<<"SELECT CASE WHEN to_regclass('public.lc_schema_migrations') IS NULL THEN -1 ELSE (SELECT count(*) FROM public.lc_schema_migrations) END;")"
[[ "$ledger" =~ ^[0-9]+$ && "$ledger" -gt 0 ]] || die "migration ledger missing or empty (run migrate first)"

# ---- 3. create / alter / grant (one psql session, logging of statements disabled) ------------
{
  # Password literals must never reach the server log (log_statement, error statements, slow log).
  echo "SET log_statement = 'none';"
  echo "SET log_min_error_statement = 'panic';"
  echo "SET log_min_duration_statement = -1;"
  echo "SET client_min_messages = 'warning';"
  for i in "${!logins[@]}"; do
    login=${logins[$i]} authority=${authorities[$i]} grant=${grants[$i]}
    echo "\\set login '$login'"
    echo "\\set authority '$authority'"
    echo "\\set pw \`cat $SECRETS/pw_$login\`"
    echo "SELECT format('CREATE ROLE %I LOGIN INHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION CONNECTION LIMIT 40', :'login') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'login') \\gexec"
    # Rotation = rewrite pw_<login>, re-run this job, restart the consumer.
    echo "SELECT format('ALTER ROLE %I WITH LOGIN INHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION CONNECTION LIMIT 40 PASSWORD %L', :'login', :'pw') \\gexec"
    echo "\\unset pw"
    if [[ "$grant" == in_role ]]; then
      # Identical to CREATE ROLE ... IN ROLE: INHERIT follows the login attribute, SET TRUE.
      echo "SELECT format('GRANT %I TO %I', :'authority', :'login') \\gexec"
    else
      echo "SELECT format('GRANT %I TO %I WITH INHERIT TRUE, SET FALSE', :'authority', :'login') \\gexec"
    fi
  done
} | su_psql >/dev/null || die "create/alter/grant failed (see postgres logs; no statement text is logged)"
log "roles created/updated: ${#logins[@]}"

# ---- 4. verify the membership matrix (exactly one authority, tested shape) -------------------
values=""
for i in "${!logins[@]}"; do
  values+="${values:+,}('${logins[$i]}','${authorities[$i]}','${grants[$i]}')"
done
report="$(su_psql <<SQL
WITH spec(login, authority, grant_shape) AS (VALUES $values),
authorities(name) AS (VALUES
  ('commerce_runtime'),('commerce_identity'),('commerce_buyer_runtime'),('commerce_buyer_issuer'),
  ('commerce_worker'),('commerce_checkout_runtime'),('commerce_meta_ingress'),('commerce_meta_registrar'),
  ('commerce_meta_curator'),('commerce_meta_consumer'),('commerce_meta_worker'),('commerce_media_registrar'),
  ('commerce_media_worker'),('commerce_media_executor'),('commerce_media_recovery')),
checked AS (
  SELECT s.login, s.authority, s.grant_shape, r.oid AS login_oid,
         r.rolcanlogin, r.rolsuper, r.rolbypassrls, r.rolcreaterole, r.rolcreatedb, r.rolreplication, r.rolinherit,
         ARRAY(SELECT a.name::text FROM authorities a
               WHERE coalesce(pg_has_role(r.oid, to_regrole(a.name), 'MEMBER'), false) ORDER BY 1) AS members,
         CASE WHEN s.authority = 'commerce_hosted_runtime' THEN ARRAY['commerce_checkout_runtime']
              ELSE ARRAY[s.authority::text] END AS expected,
         coalesce(pg_has_role(r.oid, to_regrole(s.authority), 'USAGE'), false) AS can_use,
         coalesce(pg_has_role(r.oid, to_regrole(s.authority), 'SET'), false) AS can_set,
         coalesce(pg_has_role(r.oid, to_regrole('commerce_hosted_runtime'), 'MEMBER'), false) AS hosted_member,
         coalesce(pg_has_role(r.oid, to_regrole('commerce_auth'), 'MEMBER'), false) AS auth_member,
         EXISTS (SELECT 1 FROM pg_roles w WHERE w.rolname LIKE '%\_writer' ESCAPE '\'
                 AND pg_has_role(r.oid, w.oid, 'MEMBER')) AS writer_member,
         EXISTS (SELECT 1 FROM pg_roles p WHERE p.rolname LIKE 'pg\_%' ESCAPE '\'
                 AND p.rolname <> 'pg_database_owner' AND pg_has_role(r.oid, p.oid, 'MEMBER')) AS predefined_member
  FROM spec s LEFT JOIN pg_roles r ON r.rolname = s.login)
SELECT login, authority, grant_shape,
  CASE
    WHEN login_oid IS NULL THEN 'missing_login'
    WHEN to_regrole(authority) IS NULL THEN 'missing_authority_role'
    WHEN NOT rolcanlogin OR rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb OR rolreplication OR NOT rolinherit
      THEN 'unsafe_role_attributes'
    WHEN members <> expected THEN 'membership=' || array_to_string(members, ',')
    WHEN auth_member THEN 'member_of_commerce_auth'
    WHEN writer_member THEN 'member_of_a_writer_role'
    WHEN predefined_member THEN 'member_of_a_predefined_pg_role'
    WHEN authority <> 'commerce_hosted_runtime' AND hosted_member THEN 'member_of_commerce_hosted_runtime'
    WHEN NOT can_use THEN 'authority_not_inherited'
    WHEN grant_shape = 'inherit_noset' AND can_set THEN 'set_role_allowed'
    ELSE 'ok'
  END
FROM checked ORDER BY login;
SQL
)" || die "verification query failed"
drift=0
while IFS='|' read -r login authority grant result; do
  [[ -z "$login" ]] && continue
  if [[ "$result" == ok ]]; then
    log "login=$login authority=$authority grant=$grant membership=ok"
  else
    log "DRIFT login=$login authority=$authority grant=$grant problem=$result"
    drift=1
  fi
done <<<"$report"
((drift == 0)) || die "role drift detected; fix manually (docs/runbooks/incident.md §DB 角色), never auto-repaired"

# ---- 5. every login authenticates over TCP, exactly like its service -------------------------
for login in "${logins[@]}"; do
  got="$(PGPASSWORD="$(<"$SECRETS/pw_$login")" psql -X -q -At -h "$PG_HOST" -U "$login" -d "$DB" \
    -c 'SELECT session_user' 2>/dev/null)" || die "login=$login auth=failed (host=$PG_HOST)"
  [[ "$got" == "$login" ]] || die "login=$login auth=unexpected_session_user"
  log "login=$login auth=ok"
done

# ---- 6. readiness gates (post-River migrations complete) --------------------------------------
ready="$(su_psql <<<"SELECT integration.payment_queue_ready(), checkout.expiry_queue_ready(), meta_inbox.runtime_ready(), live.media_plan_ready();")" ||
  die "readiness query failed"
IFS='|' read -r r_pay r_exp r_meta r_media <<<"$ready"
log "ready.integration.payment_queue_ready=$r_pay ready.checkout.expiry_queue_ready=$r_exp ready.meta_inbox.runtime_ready=$r_meta ready.live.media_plan_ready=$r_media"
[[ "$r_pay$r_exp$r_meta" == ttt ]] || die "a readiness gate is false: unapplied/partial migration (no auto-repair)"
# The media gate is only needed by Studio/media-worker, which are not deployable in this release
# (preflight P06 forces COMMERCE_STUDIO_ENABLED=0). It is false on every LOGICALLY restored
# database (constraint-digest pin, see deploy/postgres/ops/verify.sql), so it is required only
# with LC_REQUIRE_MEDIA_GATE=1.
if [[ "$r_media" != t ]]; then
  [[ "${LC_REQUIRE_MEDIA_GATE:-0}" == 1 ]] && die "live.media_plan_ready() is false and LC_REQUIRE_MEDIA_GATE=1"
  log "WARN live.media_plan_ready()=f (informational: Studio/media disabled; expected after a logical restore)"
fi
# payment-worker-runtime.md step 5: the gate must also be true as the worker login itself.
for i in "${!logins[@]}"; do
  [[ "${logins[$i]}" == lc_payment_sandbox ]] || continue
  got="$(PGPASSWORD="$(<"$SECRETS/pw_lc_payment_sandbox")" psql -X -q -At -h "$PG_HOST" -U lc_payment_sandbox -d "$DB" \
    -c 'SELECT integration.payment_queue_ready()' 2>/dev/null)" || die "readiness as lc_payment_sandbox failed"
  [[ "$got" == t ]] || die "integration.payment_queue_ready() false as lc_payment_sandbox"
  log "ready.as.lc_payment_sandbox=t"
done

# ---- 7. summary --------------------------------------------------------------------------------
max_version="$(su_psql <<<"SELECT max(version) FROM public.lc_schema_migrations;")"
log "summary logins=${#logins[@]} ledger_rows=$ledger ledger_max=$max_version result=ok"
