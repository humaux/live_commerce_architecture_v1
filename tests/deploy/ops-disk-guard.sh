#!/usr/bin/env bash
# File: tests/deploy/ops-disk-guard.sh
# Purpose: gates OD1-OD6 of unit ops-disk-guard (docs/delivery/units/ops-disk-guard.md): the bounded WAL archive,
#   alerts that reach a human, the disk budget guard and bounded Docker growth, after the 2026-10-03 disk-full outage.
#     pg      OD1 (REAL PG) the repo's pinned postgres image + deploy/postgres/postgresql.conf archive WAL gzip-compressed:
#                   .gz segments, avg bytes/segment < 200 KB, projected volume, idempotent re-archive, a corrupted target
#                   and a full disk fail loudly (no .part left behind)
#             OD2 (REAL PG) restore-pitr.sh (real script, real pg_basebackup, real PG replay) restores a row inserted after
#                   the base from compressed + legacy plain WAL; verify.sql is replaced by a witness-row check because the
#                   scratch database has no application schema (migrations are not applied here)
#             OD3 (REAL PG + pg_archivecleanup) real basebackup.sh retention prunes .gz AND plain segments (+ .backup labels)
#                   older than the oldest kept base, touches nothing newer, never .history
#     alerts  OD4 (MOCK SMTP: a stub curl in a Linux container) the real watchdog.sh: one mail per failing-set change,
#                   re-send after 6 h, one "recovered" mail, the password in no argv/log/state/message, SMTP down non-fatal
#             OD4b (alerts also runs it; mode smtp = only this) the REAL alert_mail() of watchdog.sh with the REAL host curl against
#                   a local mock SMTP server (python3: STARTTLS with a self-signed certificate, AUTH, MAIL/RCPT/DATA): the message
#                   arrives with CRLF headers, the credentials authenticate (also with quote/backslash in the password), certificates
#                   are verified (untrusted cert = failure), a wrong password / closed port fail without leaking either password
#     disk    OD5 the real lib.sh / pg-ops.sh / deploy.sh refuse below max(10 %, 2 GiB) free before touching Docker/PG
#     prune   OD6 (fake images in local Docker, `docker builder prune` is stubbed) prune-docker.sh keeps the new, running
#                   and previous deployed tags and removes the rest; never forces; does nothing without deployments.log
#     all     everything above
# Usage: bash tests/deploy/ops-disk-guard.sh [pg|alerts|smtp|disk|prune|all]      (via: bash scripts/dev/test-local.sh --ops-disk-guard)
# Runs as/in: a dev machine with Docker (bash 3.2 compatible on the host; the deploy scripts run in the pinned Linux image).
#   Starts only containers/volumes named lcod<pid>-* (label livecommerce.fixture) and removes all of them at exit.
# Reads env: LC_OD_IDLE_SECONDS (OD1 measurement window, default 300 = the 5 minutes of the brief),
#   LC_OD_CONF / LC_OD_OPS_DIR / LC_OD_SRC (negative controls: run the gate against the PRE-FIX postgresql.conf / ops scripts /
#   deploy tree, e.g. `git archive <base sha> deploy | tar -x -C DIR`; it must FAIL).
# Reads secrets: none. Every credential here is generated per run (openssl) and never printed.
# Writes: output/ops-disk-guard/ (override LC_OD_EVIDENCE): od1-archive-listing.txt etc. (small text only).
# Exit: 0 all PASS, 1 any FAIL, 2 usage / Docker or pinned image missing (NOT_RUN).
# Status: REAL PG for OD1-OD3, MOCK SMTP for OD4, REAL scripts with stubbed docker/df for OD5, fake images for OD6.
#   NOT covered here: the real Linux host (root, ext4, cron), a real SMTP relay, smoke full (S28-S31/S42 need root + 80/443).
set -Eeuo pipefail
case "${1:-}" in inner-*) ;; *) cd "$(git rev-parse --show-toplevel)" ;; esac # inner-* run inside the Linux image (no git)
ROOT=$(pwd)
SRC=${LC_OD_SRC:-$ROOT} # tree holding deploy/ (negative controls: a checkout of the PRE-FIX tree); the harness itself is always this file
mode=${1:-all}
EV=${LC_OD_EVIDENCE:-output/ops-disk-guard}

# ------------------------------------------------------------------------------------------------ in-container (bash 5)
if [[ "$mode" == inner-alerts || "$mode" == inner-disk ]]; then
  W=${2:?work dir}
  fails=0
  ok() { printf 'PASS %s\n' "$*"; }
  bad() { printf 'FAIL %s\n' "$*"; fails=$((fails + 1)); }
  # ck LABEL CMD... — PASS when CMD succeeds (errexit is off inside the if).
  ck() { local l=$1; shift; if "$@"; then ok "$l"; else bad "$l"; fi; }
  set +e
fi

# ---------------------------------------------------------------------------------------------------------- OD4 (inner)
if [[ "$mode" == inner-alerts ]]; then
  mkdir -p "$W/bin" "$W/state" "$W/okdisk" "$W/secrets"
  pw=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n') # fake relay password, generated per run, never printed
  printf '%s\n' "$pw" >"$W/secrets/commerce_smtp_password"
  chmod 0440 "$W/secrets/commerce_smtp_password"
  echo running >"$W/api.state"
  cat >"$W/bin/docker" <<'STUB'
#!/usr/bin/env bash
# Stub docker: one long-running service "api" whose state is the first line of $W/api.state; no postgres, no caddy.
args=" $* "
case "$args" in
*" info "*) echo /dev/shm ;;
*" inspect "*)
  case "$args" in
  *State.Status*) printf '%s|none|0\n' "$(cat "$W/api.state")" ;;
  *) echo 'postgres:18' ;;
  esac ;;
*" compose "*)
  case "$args" in
  *" config --services "*) echo api ;;
  *" ps -q "*) echo cid-api ;;
  esac ;;
esac
exit 0
STUB
  cat >"$W/bin/df" <<'STUB'
#!/usr/bin/env bash
# Stub df: any path under */full* reports 96 % used; everything else is the real df.
case "$*" in
*/full*) printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\nfake 100000000 96000000 4000000 96%% /full\n' ;;
*) exec /usr/bin/df "$@" ;;
esac
STUB
  cat >"$W/bin/curl" <<'STUB'
#!/usr/bin/env bash
# Stub curl = the MOCK SMTP relay. Mail call (has --mail-rcpt): log argv, keep the -K config (a pipe /dev/fd/N or stdin) and the
# message (-T - = stdin), fail when $W/smtp.down exists. Webhook call (has -d): keep the body.
printf '%s\n' "$*" >>"$W/curl.argv"
mail=0 cfgsrc="" msgsrc=""
args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  case "${args[i]}" in
  --mail-rcpt) mail=1 ;;
  -d) printf '%s' "${args[i + 1]}" >"$W/hook.body" ;;
  -T) msgsrc=${args[i + 1]} ;;
  -K) cfgsrc=${args[i + 1]} ;;
  esac
done
if ((mail)); then
  n=$(($(cat "$W/mail.attempts" 2>/dev/null || echo 0) + 1))
  echo "$n" >"$W/mail.attempts"
  [[ -n "$cfgsrc" && "$cfgsrc" != - ]] && cat "$cfgsrc" >"$W/curl.stdin.$n"
  [[ "$msgsrc" == - ]] && cat >"$W/msg.in"
  if [[ -e "$W/smtp.down" ]]; then
    echo 'curl: (7) Failed to connect to smtp.fake.invalid port 465: Could not connect to server' >&2
    exit 7
  fi
  c=$(($(cat "$W/mail.count" 2>/dev/null || echo 0) + 1)) # delivered messages (attempts that failed are not counted)
  echo "$c" >"$W/mail.count"
  cp "$W/msg.in" "$W/mail.$c.eml"
fi
exit 0
STUB
  chmod +x "$W/bin/"*
  cat >"$W/compose.env" <<EOF
COMPOSE_PROJECT_NAME=odw
COMPOSE_PROFILES=app
LC_ENVIRONMENT=smoke
IMAGE_TAG=smoke
LC_SECRETS_DIR=$W/secrets
LC_STATE_DIR=$W/state
LC_BACKUP_DIR=$W/okdisk
LC_API_HOST=api.fake.invalid
LC_SMTP_HOST=smtp.fake.invalid
LC_SMTP_USERNAME=alerts-sender@fake.invalid
LC_MAIL_FROM=alerts <alerts-sender@fake.invalid>
LC_ALERT_EMAIL=owner@fake.invalid
EOF
  n=0
  # wd [NAME=VALUE env overrides...] [--script-args...] — one real watchdog.sh run; $W/out.N / $W/err.N, status in $rc.
  wd() {
    local envs=() args=() a
    n=$((n + 1))
    for a in "$@"; do if [[ "$a" == --* ]]; then args+=("$a"); else envs+=("$a"); fi; done
    env -i PATH="$W/bin:/usr/bin:/bin" HOME="$W" W="$W" LC_COMPOSE_ENV="$W/compose.env" LC_CONFIG_DIR="$W" "${envs[@]}" \
      bash /repo/deploy/scripts/watchdog.sh "${args[@]}" >"$W/out.$n" 2>"$W/err.$n"
    rc=$?
  }
  mails() { cat "$W/mail.count" 2>/dev/null || echo 0; }
  subj() { sed -n "s/^Subject: //p" "$W/mail.$1.eml" | tr -d '\r'; }
  state() { cat "$W/state/watchdog.state" 2>/dev/null; }
  set_state() { # KEY VALUE
    sed -i "/^$1\t/d" "$W/state/watchdog.state"
    printf '%s\t%s\n' "$1" "$2" >>"$W/state/watchdog.state"
  }

  # 1. first failure (W1: api exited): exit 1, ONE mail naming W1
  echo exited >"$W/api.state"
  wd
  ck "OD4 1 first failure: exit 1" test "$rc" = 1
  ck "OD4 1 first failure: exactly one mail" test "$(mails)" = 1
  ck "OD4 1 subject names the failing set" grep -q '^\[live-commerce\] ALERT: W1$' <(subj 1)
  ck "OD4 1 body carries the value-free FAIL line" grep -q '^W1 FAIL api state=exited' "$W/mail.1.eml"
  ck "OD4 1 recipient and sender" bash -c "grep -q '^To: owner@fake.invalid' '$W/mail.1.eml' && grep -q '^From: alerts-sender@fake.invalid' '$W/mail.1.eml'"
  ck "OD4 1 mailed through smtps on 465, config from a pipe (-K /dev/fd/N), message on stdin (-T -)" bash -c "grep -q -- '--url smtps://smtp.fake.invalid:465' '$W/curl.argv' && grep -qE -- '-K /dev/fd/[0-9]+ ' '$W/curl.argv' && grep -q -- ' -T -' '$W/curl.argv'"
  ck "OD4 1 state remembers the set" grep -q $'^alert_set\tW1$' <(state)
  # 2. same failing set again: no mail
  wd
  ck "OD4 2 unchanged set: still exit 1, no new mail" bash -c "[ $rc = 1 ] && [ $(mails) = 1 ]"
  # 3. the set changes (W2 joins): one new mail at once
  mkdir -p "$W/full"
  wd LC_BACKUP_DIR="$W/full"
  ck "OD4 3 changed set: second mail" test "$(mails)" = 2
  ck "OD4 3 subject lists both checks" grep -q '^\[live-commerce\] ALERT: W1 W2$' <(subj 2)
  wd LC_BACKUP_DIR="$W/full"
  ck "OD4 3b unchanged again: no new mail" test "$(mails)" = 2
  # 4. 6 h later with the same set (injected state clock): re-send once
  set_state alert_sent $(($(date +%s) - 7 * 3600))
  wd LC_BACKUP_DIR="$W/full"
  ck "OD4 4 after 6 h: third mail" test "$(mails)" = 3
  ck "OD4 4 marked as still failing" grep -q 'still failing' <(subj 3)
  set_state alert_sent $(($(date +%s) - 5 * 3600))
  wd LC_BACKUP_DIR="$W/full"
  ck "OD4 4b only 5 h later: no mail" test "$(mails)" = 3
  # 5. everything passes again: exactly one recovered mail, exit 0, state cleared
  echo running >"$W/api.state"
  wd
  ck "OD4 5 recovery: exit 0" test "$rc" = 0
  ck "OD4 5 recovery: one RECOVERED mail" bash -c "[ $(mails) = 4 ] && grep -q '^\[live-commerce\] RECOVERED' <(sed -n 's/^Subject: //p' '$W/mail.4.eml')"
  ck "OD4 5 state cleared" bash -c "! grep -q '^alert_' '$W/state/watchdog.state'"
  wd
  ck "OD4 5b all-pass stays silent" test "$(mails)" = 4
  # 6. SMTP down: never fatal, state does not advance, the next run retries and sends once
  : >"$W/smtp.down"
  echo exited >"$W/api.state"
  wd
  ck "OD4 6 SMTP down: watchdog result unchanged (exit 1 from W1), warning logged" bash -c "[ $rc = 1 ] && grep -q 'alert mail not sent (curl exit 7)' '$W/err.$n'"
  ck "OD4 6 SMTP down: no mail counted, state not advanced" bash -c "[ $(mails) = 4 ] && ! grep -q '^alert_' '$W/state/watchdog.state'"
  rm -f "$W/smtp.down"
  wd
  ck "OD4 6 SMTP back: the retry sends exactly one mail" test "$(mails)" = 5
  # 6b. recovery mail while SMTP is down: exit 0 (non-fatal), retried later
  : >"$W/smtp.down"
  echo running >"$W/api.state"
  wd
  ck "OD4 6b RECOVERED mail fails: exit still 0, set kept for the retry" bash -c "[ $rc = 0 ] && grep -q $'^alert_set\tW1' '$W/state/watchdog.state'"
  rm -f "$W/smtp.down"
  wd
  ck "OD4 6b retry sends the recovered mail" bash -c "[ $(mails) = 6 ] && grep -q '^\[live-commerce\] RECOVERED' <(sed -n 's/^Subject: //p' '$W/mail.6.eml')"
  # 7. webhook + e-mail coexist
  echo exited >"$W/api.state"
  wd LC_ALERT_WEBHOOK_URL=http://hook.fake.invalid/x
  ck "OD4 7 webhook still posts the failing ids" grep -q '"failed":"W1"' "$W/hook.body"
  ck "OD4 7 and the e-mail went out" test "$(mails)" = 7
  # 8. misconfiguration is a warning, never a failure of the watchdog
  echo running >"$W/api.state"
  wd LC_ALERT_EMAIL= # reset the failing state without mailing
  echo exited >"$W/api.state"
  wd LC_ALERT_EMAIL='bad address@x'
  ck "OD4 8 malformed LC_ALERT_EMAIL: warning, no mail attempt, exit 1" bash -c "[ $rc = 1 ] && [ $(mails) = 7 ] && grep -q 'LC_ALERT_EMAIL is not a plain address' '$W/err.$n'"
  wd LC_ALERT_EMAIL=
  ck "OD4 8 LC_ALERT_EMAIL unset: behaves as before (no mail, exit 1)" bash -c "[ $rc = 1 ] && [ $(mails) = 7 ]"
  # 8b. --test-mail: one TEST message through the same path, no checks run; failing SMTP is an explicit failure here
  before=$(mails)
  wd --test-mail
  ck "OD4 8b --test-mail sends exactly one TEST mail and exits 0 without running checks" bash -c "[ $rc = 0 ] && [ $(mails) = $((before + 1)) ] && grep -q '^Subject: \[live-commerce\] TEST' '$W/mail.$((before + 1)).eml' && ! grep -q '^W1 ' '$W/out.$n'"
  : >"$W/smtp.down"
  wd --test-mail
  ck "OD4 8b --test-mail with the relay down exits 1 (the operator asked for exactly this)" test "$rc" = 1
  rm -f "$W/smtp.down"
  wd --bogus
  ck "OD4 8b unknown argument: usage error (exit 2)" test "$rc" = 2
  # 9. the password: sent to curl on stdin, and nowhere else
  ck "OD4 9 the credential did reach curl (via the -K pipe)" grep -q "^user = \"alerts-sender@fake.invalid:$pw\"$" "$W/curl.stdin.1"
  ck "OD4 9 password in NO curl argv" bash -c "! grep -qF '$pw' '$W/curl.argv'"
  ck "OD4 9 password in NO watchdog output or log" bash -c "! cat '$W'/out.* '$W'/err.* | grep -qF '$pw'"
  ck "OD4 9 password in NO state file" bash -c "! grep -qF '$pw' '$W/state/watchdog.state'"
  ck "OD4 9 password in NO mail message" bash -c "! cat '$W'/mail.*.eml | grep -qF '$pw'"
  ck "OD4 9 no argv carries --user/-u" bash -c "! grep -qE -- '(^| )(--user|-u) ' '$W/curl.argv'"
  printf 'OD4 summary: watchdog runs=%s mails=%s fails=%s\n' "$n" "$(mails)" "$fails"
  exit $((fails > 0))
fi

# ---------------------------------------------------------------------------------------------------------- OD5 (inner)
if [[ "$mode" == inner-disk ]]; then
  mkdir -p "$W/deploy" "$W/bin" "$W/state" "$W/backup"
  cp -R /repo/deploy/scripts "$W/deploy/scripts"
  printf '#!/usr/bin/env bash\nexit 0\n' >"$W/deploy/scripts/preflight.sh" # deploy.sh upgrade runs preflight first: not under test here
  cat >"$W/bin/docker" <<'STUB'
#!/usr/bin/env bash
# Stub docker: records every call; images "exist"; any compose call touching pg-ops or postgres fails (so a flow that
# gets past the guard ends there instead of running on).
printf '%s\n' "$*" >>"$W/docker.calls"
case "$*" in
*" image inspect "* | "image inspect "*) exit 0 ;;
*pg-ops*) exit 99 ;;
esac
exit 0
STUB
  cat >"$W/bin/df" <<'STUB'
#!/usr/bin/env bash
# Stub df: size/avail (KiB) from $W/df.size / $W/df.avail for every path; the real df when absent.
if [[ -f "$W/df.size" ]]; then
  printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\nfake %s 1 %s 1%% /x\n' "$(cat "$W/df.size")" "$(cat "$W/df.avail")"
else exec /usr/bin/df "$@"; fi
STUB
  chmod +x "$W/bin/"* "$W/deploy/scripts/"*.sh
  cat >"$W/compose.env" <<EOF
COMPOSE_PROJECT_NAME=odd
COMPOSE_PROFILES=db,app
LC_ENVIRONMENT=smoke
IMAGE_TAG=smoke
LC_BACKUP_DIR=$W/backup
LC_STATE_DIR=$W/state
EOF
  setdf() { echo "$1" >"$W/df.size"; echo "$2" >"$W/df.avail"; }
  GIB=1048576 # KiB per GiB
  # shellcheck disable=SC1091
  source "$W/deploy/scripts/lib.sh"
  guard() { (lc_disk_guard "$W/backup") >/dev/null 2>&1; }
  export PATH="$W/bin:$PATH" W
  # thresholds: max(10 % of the filesystem, 2 GiB)
  setdf $((100 * GIB)) $((10 * GIB)); ck "OD5 100 GiB fs, exactly 10 % free: allowed" guard
  setdf $((100 * GIB)) $((10 * GIB - 1))
  if guard; then bad "OD5 100 GiB fs, 1 KiB under 10 % refused"; else ok "OD5 100 GiB fs, 1 KiB under 10 % refused"; fi
  setdf $((10 * GIB)) $((2 * GIB)); ck "OD5 10 GiB fs, exactly 2 GiB free (floor beats 10 %): allowed" guard
  setdf $((10 * GIB)) $((2 * GIB - 1)); if guard; then bad "OD5 10 GiB fs, 1 KiB under the 2 GiB floor refused"; else ok "OD5 10 GiB fs, 1 KiB under the 2 GiB floor refused"; fi
  setdf $((96 * GIB)) $((GIB / 2)); if guard; then bad "OD5 the incident (96 GiB fs, 0.5 GiB free) refused"; else ok "OD5 the incident (96 GiB fs, 0.5 GiB free) refused"; fi
  # the refusal message is the clear one
  msg=$( (lc_disk_guard "$W/backup") 2>&1 || true)
  ck "OD5 message names path, free space and the rule" grep -q "disk guard: $W/backup has 512 MiB free of 98304 MiB; refusing to write (need max(10 %, 2 GiB) = 9830 MiB)" <<<"$msg"
  # pg-ops.sh backup / basebackup and deploy.sh upgrade, space exhausted: refuse BEFORE docker / PG
  run() { : >"$W/docker.calls"; LC_COMPOSE_ENV="$W/compose.env" LC_CONFIG_DIR="$W" "$@" >"$W/run.out" 2>&1; rc=$?; }
  setdf $((96 * GIB)) $((GIB / 2))
  run bash "$W/deploy/scripts/pg-ops.sh" backup --tag t
  ck "OD5 pg-ops backup refuses (exit 1, guard message, docker never called)" bash -c "[ $rc = 1 ] && grep -q 'disk guard' '$W/run.out' && [ ! -s '$W/docker.calls' ]"
  run bash "$W/deploy/scripts/pg-ops.sh" basebackup
  ck "OD5 pg-ops basebackup refuses (exit 1, guard message, docker never called)" bash -c "[ $rc = 1 ] && grep -q 'disk guard' '$W/run.out' && [ ! -s '$W/docker.calls' ]"
  run bash "$W/deploy/scripts/deploy.sh" upgrade tagx
  ck "OD5 deploy.sh upgrade refuses before the backup (exit 1, guard message, no compose/pg-ops call, no rollback tree)" \
    bash -c "[ $rc = 1 ] && grep -q 'disk guard' '$W/run.out' && ! grep -q compose '$W/docker.calls' && ! grep -q 'Deploy step failed' '$W/run.out'"
  # positive controls: with room the same commands get past the guard and reach docker (the stub then stops them)
  setdf $((100 * GIB)) $((50 * GIB))
  run bash "$W/deploy/scripts/pg-ops.sh" backup --tag t
  ck "OD5 positive control: pg-ops backup with room reaches docker compose (pg-ops)" bash -c "! grep -q 'disk guard' '$W/run.out' && grep -q 'pg-ops' '$W/docker.calls'"
  run bash "$W/deploy/scripts/deploy.sh" upgrade tagx
  ck "OD5 positive control: deploy.sh upgrade with room reaches the backup step (pg-ops)" bash -c "! grep -q 'disk guard' '$W/run.out' && grep -q 'pg-ops' '$W/docker.calls'"
  # real GNU df on a real tiny tmpfs (the container is started with --tmpfs /small:size=8m)
  rm -f "$W/df.size" "$W/df.avail"
  real=$( (lc_disk_guard /small) 2>&1 || true)
  ck "OD5 real tmpfs (8 MiB): refused with the real df numbers" grep -q 'disk guard: /small has [0-9]* MiB free of 8 MiB' <<<"$real"
  ck "OD5 unreadable path: refused" bash -c "! (lc_disk_guard /nonexistent-od5) >/dev/null 2>&1"
  printf 'OD5 summary: fails=%s\n' "$fails"
  exit $((fails > 0))
fi

# ------------------------------------------------------------------------------------------------------------ host side
case "$mode" in pg | alerts | smtp | disk | prune | all) ;; *)
  echo "usage: tests/deploy/ops-disk-guard.sh [pg|alerts|smtp|disk|prune|all]" >&2
  exit 2
  ;;
esac
command -v docker >/dev/null && docker info >/dev/null 2>&1 || {
  echo "NOT_RUN: Docker is not available" >&2
  exit 2
}
IMG=$(grep -o 'postgres@sha256:[0-9a-f]\{64\}' deploy/compose.yml | head -n1)
docker image inspect "$IMG" >/dev/null 2>&1 || {
  echo "NOT_RUN: pinned image $IMG is not present locally (the gate never pulls)" >&2
  exit 2
}
RUN="lcod$$"
TMP=$(mktemp -d "${TMPDIR:-/tmp}/lc-od-XXXXXX")
mkdir -p "$EV"
fails=0
ok() { printf 'PASS %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fails=$((fails + 1)); }
ck() { local l=$1; shift; if "$@"; then ok "$l"; else bad "$l"; fi; }
cleanup() {
  local c
  for c in $(docker ps -aq --filter "label=livecommerce.fixture=$RUN" 2>/dev/null); do docker rm -f "$c" >/dev/null 2>&1 || true; done
  for c in $(docker volume ls -q --filter "label=livecommerce.fixture=$RUN" 2>/dev/null); do docker volume rm -f "$c" >/dev/null 2>&1 || true; done
  for c in $(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep "^${RUN}[a-z]*-" || true); do docker rmi "$c" >/dev/null 2>&1 || true; done
  rm -rf "$TMP"
}
trap cleanup EXIT
set +e # from here every check is explicit (ck/bad); a failing probe must be recorded, not abort the run
secret() { od -An -N24 -tx1 /dev/urandom | tr -d ' \n'; }

# ---------------------------------------------------------------------------------------------------------- OD1-OD3
od_pg() {
  local conf=${LC_OD_CONF:-deploy/postgres/postgresql.conf} opsdir=${LC_OD_OPS_DIR:-deploy/postgres/ops}
  local idle=${LC_OD_IDLE_SECONDS:-300} PG="$RUN-pg" v out
  local pw
  [[ "$conf" == /* ]] || conf="$ROOT/$conf"
  [[ "$opsdir" == /* ]] || opsdir="$ROOT/$opsdir"
  pw=$(secret)
  printf '%s' "$pw" >"$TMP/pw"
  chmod 0644 "$TMP/pw" # a throwaway value read by UID 999 through the bind mount
  # pg-ops /ops = the ops scripts under test + a stub verify.sql (witness rows; the scratch DB has no application schema)
  rm -rf "$TMP/ops" && mkdir -p "$TMP/ops" && cp "$opsdir"/*.sh "$TMP/ops/"
  cat >"$TMP/ops/verify.sql" <<'SQL'
\set ON_ERROR_STOP 1
DO $$ BEGIN
  IF (SELECT array_agg(id ORDER BY id) FROM od_pitr.marker) IS DISTINCT FROM ARRAY[1, 2, 3] THEN
    RAISE EXCEPTION 'PITR witness mismatch: rows after the base must be present up to the target and none after it';
  END IF;
END $$;
\echo verify.result=PASS witness=1,2,3 (stub verify.sql: no application schema in this scratch DB)
SQL
  for v in wal bk data sock scratch; do docker volume create --label "livecommerce.fixture=$RUN" "$RUN-$v" >/dev/null; done
  docker run --rm --pull=never --user 0 --entrypoint sh -v "$RUN-wal:/w" -v "$RUN-bk:/b" "$IMG" \
    -c 'mkdir -p /b/base /b/dumps && chown -R 999:999 /w /b && chmod 700 /w /b' >/dev/null
  docker run -d --pull=never --name "$PG" --label "livecommerce.fixture=$RUN" --user 999:999 --read-only --cap-drop ALL \
    --security-opt no-new-privileges --memory 3g --shm-size 512m --tmpfs /tmp:rw,noexec,nosuid,nodev,size=64m \
    -e POSTGRES_USER=postgres -e POSTGRES_DB=live_commerce -e POSTGRES_PASSWORD_FILE=/run/secrets/pg_superuser_password \
    -e PGDATA=/var/lib/postgresql/18/docker -e POSTGRES_INITDB_ARGS=--data-checksums \
    -v "$RUN-data:/var/lib/postgresql" -v "$RUN-sock:/var/run/postgresql" -v "$RUN-wal:/backup/wal" \
    -v "$conf:/etc/postgresql/postgresql.conf:ro" -v "$ROOT/deploy/postgres/pg_hba.conf:/etc/postgresql/pg_hba.conf:ro" \
    -v "$ROOT/deploy/postgres/archive-wal.sh:/etc/postgresql/archive-wal.sh:ro" -v "$TMP/pw:/run/secrets/pg_superuser_password:ro" \
    "$IMG" postgres -c config_file=/etc/postgresql/postgresql.conf -c hba_file=/etc/postgresql/pg_hba.conf >/dev/null
  pgx() { docker exec -i "$PG" bash -c 'PGPASSWORD="$(< /run/secrets/pg_superuser_password)" exec psql -X -q -At -F "|" -v ON_ERROR_STOP=1 -h /var/run/postgresql -U postgres -d live_commerce'; }
  local i
  for ((i = 0; i < 90; i++)); do
    docker exec "$PG" bash -c 'pg_isready -q -h /var/run/postgresql -U postgres -d live_commerce && [ -n "$(sed -n 6p "$PGDATA/postmaster.pid" 2>/dev/null)" ]' 2>/dev/null && break
    sleep 2
  done
  pgx <<<"SELECT 1;" >/dev/null || { bad "OD1 postgres did not start (docker logs $PG)"; docker logs "$PG" 2>&1 | tail -n 20; return; }
  echo "info: PG $(pgx <<<"SHOW server_version;") archive_timeout=$(pgx <<<"SHOW archive_timeout;") archive_command=$(pgx <<<"SHOW archive_command;")"
  pgx <<<"CREATE SCHEMA od_pitr; CREATE TABLE od_pitr.marker(id int PRIMARY KEY, note text); CREATE TABLE od_pitr.noise(ts timestamptz DEFAULT now(), n int); INSERT INTO od_pitr.marker VALUES (1, 'before-base');"
  arch() { pgx <<<"SELECT archived_count, failed_count, coalesce(last_archived_wal, '') FROM pg_stat_archiver;"; }
  # wait_archived WALFILE — until pg_stat_archiver.last_archived_wal >= WALFILE (same timeline: names sort).
  wait_archived() {
    local want=$1 last
    for ((i = 0; i < 90; i++)); do
      last=$(pgx <<<"SELECT coalesce(last_archived_wal, '') FROM pg_stat_archiver;")
      [[ "$last" > "$want" || "$last" == "$want" ]] && return 0
      sleep 1
    done
    return 1
  }
  ops() { # one pg-ops-shaped container (compose.yml pg-ops: UID 999, read-only, no network, socket + backup + scratch volumes)
    docker run --rm --pull=never --label "livecommerce.fixture=$RUN" --user 999:999 --read-only --cap-drop ALL \
      --security-opt no-new-privileges --network none --memory 1g --shm-size 256m --tmpfs /tmp:rw,noexec,nosuid,nodev,size=256m \
      -v "$RUN-sock:/var/run/postgresql" -v "$RUN-bk:/backup" -v "$RUN-wal:/backup/wal" -v "$TMP/ops:/ops:ro" \
      -v "$RUN-scratch:/var/lib/postgresql" -v "$TMP/pw:/run/secrets/pg_superuser_password:ro" --entrypoint bash "$IMG" "$@"
  }
  # switch_name — switch the WAL segment (with a commit first, so it really switches) and print the name of the closed one.
  switch_name() {
    local n
    pgx <<<"INSERT INTO od_pitr.noise(n) VALUES (0);" >/dev/null
    n=$(pgx <<<"SELECT pg_walfile_name(pg_current_wal_lsn());")
    pgx <<<"SELECT pg_switch_wal();" >/dev/null
    echo "$n"
  }
  walls() { docker exec "$PG" bash -c 'cd /backup/wal && for f in *; do [ -e "$f" ] && printf "%s %s\n" "$f" "$(stat -c %s "$f")"; done' | sort; }

  # ---- OD1: idle DB with archive_timeout=60s. A background writer does what the app's workers do: a tiny commit every 5 s,
  # so every minute has WAL activity and archive_timeout forces a (mostly zero) 16 MiB segment.
  # Bootstrap boundary: initdb + CREATE DATABASE + the schema above write real data into the first segment(s) (~2.6 MB compressed),
  # which says nothing about steady state. Close that segment first and measure only the segments created AFTER it.
  local t0 elapsed boundary
  boundary=$(switch_name)
  wait_archived "$boundary" || bad "OD1 the bootstrap segment was not archived"
  # PG 18.6 (verified with archive_timeout=5s/10s on the pinned image): archive_timeout only starts forcing switches after the
  # server's first checkpoint since startup (the first one is checkpoint_timeout = 10 min away). The pilot had been up for days, so
  # take that checkpoint now, like a server that is past its first checkpoint.
  pgx <<<"CHECKPOINT;" >/dev/null
  t0=$(date +%s)
  printf 'DO $$ BEGIN FOR i IN 1..%s LOOP INSERT INTO od_pitr.noise(n) VALUES (i); COMMIT; PERFORM pg_sleep(5); END LOOP; END $$;\n' "$((idle / 5 + 60))" |
    docker exec -i "$PG" sh -c 'cat >/tmp/noise.sql'
  docker exec -d "$PG" bash -c 'PGPASSWORD="$(< /run/secrets/pg_superuser_password)" exec psql -X -q -h /var/run/postgresql -U postgres -d live_commerce -f /tmp/noise.sql'
  echo "info: OD1 measuring for ${idle}s (trickle writer + 3 forced switches); started $(date -u +%H:%M:%SZ)"
  for ((i = 0; i < 3; i++)); do
    sleep $((idle / 6))
    switch_name >/dev/null
  done
  while (($(date +%s) - t0 < idle)); do sleep 5; done
  elapsed=$(($(date +%s) - t0))
  local cur
  cur=$(switch_name)
  wait_archived "$cur" || bad "OD1 the archiver did not catch up (see archive_status below)"
  walls >"$EV/od1-archive-listing.txt"
  local segs gz plain total avg boot_n boot_bytes
  segs=$(awk -v b="$boundary" '/^[0-9A-F]{24}(\.gz)? / { if (substr($1, 1, 24) > b) n++ } END { print n + 0 }' "$EV/od1-archive-listing.txt")
  gz=$(awk -v b="$boundary" '/^[0-9A-F]{24}\.gz / { if (substr($1, 1, 24) > b) n++ } END { print n + 0 }' "$EV/od1-archive-listing.txt")
  plain=$(awk -v b="$boundary" '/^[0-9A-F]{24} / { if (substr($1, 1, 24) > b) n++ } END { print n + 0 }' "$EV/od1-archive-listing.txt")
  total=$(awk -v b="$boundary" '/^[0-9A-F]{24}(\.gz)? / { if (substr($1, 1, 24) > b) s += $2 } END { print s + 0 }' "$EV/od1-archive-listing.txt")
  boot_n=$(awk -v b="$boundary" '/^[0-9A-F]{24}(\.gz)? / { if (substr($1, 1, 24) <= b) n++ } END { print n + 0 }' "$EV/od1-archive-listing.txt")
  boot_bytes=$(awk -v b="$boundary" '/^[0-9A-F]{24}(\.gz)? / { if (substr($1, 1, 24) <= b) s += $2 } END { print s + 0 }' "$EV/od1-archive-listing.txt")
  avg=$((segs > 0 ? total / segs : 0))
  echo "od1.bootstrap_segments_excluded=$boot_n bytes=$boot_bytes (initdb + schema: real data, not steady state)"
  echo "od1.segments=$segs gz=$gz plain=$plain total_bytes=$total avg_bytes_per_segment=$avg window_s=$elapsed"
  awk -v avg="$avg" -v segs="$segs" -v el="$elapsed" 'BEGIN { printf "od1.projected_at_1_segment_per_minute_gb_per_day=%.4f (uncompressed 16 MiB: %.2f)\nod1.projected_at_measured_rate_gb_per_day=%.4f\n", avg * 1440 / 1e9, 16777216 * 1440 / 1e9, avg * (segs / el) * 86400 / 1e9 }'
  ck "OD1 enough post-bootstrap segments archived to measure (>= 5)" test "$segs" -ge 5
  ck "OD1 every archived WAL segment is compressed (.gz), none plain" bash -c "[ $gz = $segs ] && [ $plain = 0 ]"
  ck "OD1 average archived bytes per segment < 200 KB (204800)" test "$avg" -lt 204800
  ck "OD1 archiver failed_count = 0" test "$(arch | cut -d'|' -f2)" = 0
  ck "OD1 archive_timeout is still 60s (RPO unchanged)" test "$(pgx <<<"SHOW archive_timeout;")" = 1min
  # idempotent re-archive + corrupted target, on a segment that is still in pg_wal and archived
  local seg
  seg=$(docker exec "$PG" bash -c 'cd "$PGDATA/pg_wal" && for f in $(ls | grep -E "^[0-9A-F]{24}$" | sort -r); do [ -e archive_status/$f.done ] && [ -e /backup/wal/$f.gz ] && echo $f && break; done')
  if [[ -z "$seg" ]]; then
    bad "OD1 no archived segment is still in pg_wal to re-archive"
  else
    local before after rc
    before=$(docker exec "$PG" bash -c "cd /backup/wal && ls -l --time-style=+%s | grep -v '^total' | sort")
    # archive-wal.sh is run from PGDATA with %p/%f, exactly like the archiver does
    docker exec -w /var/lib/postgresql/18/docker "$PG" bash /etc/postgresql/archive-wal.sh "pg_wal/$seg" "$seg"; rc=$?
    after=$(docker exec "$PG" bash -c "cd /backup/wal && ls -l --time-style=+%s | grep -v '^total' | sort")
    ck "OD1 re-archiving an already archived segment: exit 0" test "$rc" = 0
    ck "OD1 ... and nothing changed (no duplicate, no rewrite, no .part)" test "$before" = "$after"
    docker exec "$PG" bash -c "cp /backup/wal/$seg.gz /tmp/$seg.keep && printf '\\377\\377\\377\\377' | dd of=/backup/wal/$seg.gz bs=1 seek=200 conv=notrunc 2>/dev/null"
    docker exec -w /var/lib/postgresql/18/docker "$PG" bash /etc/postgresql/archive-wal.sh "pg_wal/$seg" "$seg" 2>"$TMP/corrupt.err"; rc=$?
    ck "OD1 a corrupted existing target is detected (non-zero, named on stderr)" bash -c "[ $rc != 0 ] && grep -q 'different or unreadable content' '$TMP/corrupt.err'"
    docker exec "$PG" bash -c "head -c 1000 /tmp/$seg.keep > /backup/wal/$seg.gz"
    docker exec -w /var/lib/postgresql/18/docker "$PG" bash /etc/postgresql/archive-wal.sh "pg_wal/$seg" "$seg" 2>/dev/null; rc=$?
    ck "OD1 a truncated existing target is detected (non-zero)" test "$rc" != 0
    docker exec "$PG" bash -c "cp /tmp/$seg.keep /backup/wal/$seg.gz && rm -f /tmp/$seg.keep"
    docker exec -w /var/lib/postgresql/18/docker "$PG" bash /etc/postgresql/archive-wal.sh "pg_wal/$seg" "$seg" 2>/dev/null; rc=$?
    ck "OD1 ... and the restored target is accepted again" test "$rc" = 0
    # a legacy plain copy of the same segment counts as archived (upgrade overlap), and a different plain copy fails
    docker exec "$PG" bash -c "mv /backup/wal/$seg.gz /tmp/$seg.gz && cp \"\$PGDATA/pg_wal/$seg\" /backup/wal/$seg"
    docker exec -w /var/lib/postgresql/18/docker "$PG" bash /etc/postgresql/archive-wal.sh "pg_wal/$seg" "$seg" 2>/dev/null; rc=$?
    ck "OD1 a legacy plain copy of the same segment is idempotent too" test "$rc" = 0
    docker exec "$PG" bash -c "rm /backup/wal/$seg && mv /tmp/$seg.gz /backup/wal/$seg.gz"
  fi
  # a full disk fails cleanly: no half-written .part left behind (the incident class), and succeeds again once there is room
  docker exec "$PG" bash -c 'mkdir -p /tmp/src /tmp/arch && head -c 16777216 /dev/urandom >/tmp/src/000000010000000000000099 && head -c 44000000 /dev/urandom >/tmp/fill'
  docker exec -e LC_WAL_ARCHIVE_DIR=/tmp/arch "$PG" bash /etc/postgresql/archive-wal.sh /tmp/src/000000010000000000000099 000000010000000000000099 2>"$TMP/full.err"; rc=$?
  ck "OD1 disk full: archive command fails (non-zero)" test "$rc" != 0
  ck "OD1 disk full: no .part and no target left behind" test -z "$(docker exec "$PG" ls /tmp/arch)"
  docker exec "$PG" rm -f /tmp/fill
  docker exec -e LC_WAL_ARCHIVE_DIR=/tmp/arch "$PG" bash /etc/postgresql/archive-wal.sh /tmp/src/000000010000000000000099 000000010000000000000099 2>/dev/null; rc=$?
  out=$(docker exec "$PG" ls /tmp/arch)
  ck "OD1 ... and the same segment archives once there is room" test "$rc/$out" = "0/000000010000000000000099.gz"
  docker exec "$PG" rm -rf /tmp/src /tmp/arch

  # ---- OD2: PITR from base + compressed WAL + legacy plain segments
  echo "info: OD2 base backup #1"
  pgx <<<"CHECKPOINT;" >/dev/null
  ops /ops/basebackup.sh >"$EV/od2-basebackup1.log" 2>&1 || bad "OD2 basebackup.sh #1 failed (see $EV/od2-basebackup1.log)"
  local base1 start1
  base1=$(ops -c 'ls /backup/base | grep -E "^20[0-9]{6}T[0-9]{6}Z$" | sort | tail -n1')
  start1=$(ops -c "tar -xzOf /backup/base/$base1/base.tar.gz backup_label | sed -n 's/^START WAL LOCATION: .* (file \\([0-9A-F]\\{24\\}\\))\$/\\1/p'")
  echo "info: base1=$base1 start_segment=$start1"
  ck "OD2 the base backup's .backup label was archived as a plain file" test -n "$(docker exec "$PG" bash -c "ls /backup/wal/${start1}.*.backup 2>/dev/null")"
  pgx <<<"INSERT INTO od_pitr.marker VALUES (2, 'after-base'), (3, 'before-target');" >/dev/null
  switch_name >/dev/null
  sleep 2
  local target
  target=$(pgx <<<"SELECT to_char(clock_timestamp() AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US') || '+00';")
  sleep 2
  pgx <<<"INSERT INTO od_pitr.marker VALUES (4, 'after-target');" >/dev/null
  cur=$(switch_name)
  wait_archived "$cur" || bad "OD2 the archiver did not catch up before the drill"
  # make the archive MIXED like the pilot's after the upgrade: every other segment from the base's start on is legacy plain
  ops -c "i=0; for f in \$(ls /backup/wal | grep -E '^[0-9A-F]{24}\\.gz\$' | sort); do n=\${f%.gz}; if [[ \$n > $start1 || \$n == $start1 ]]; then i=\$((i+1)); if ((i % 2 == 1)); then gzip -dc /backup/wal/\$f >/backup/wal/\$n && rm /backup/wal/\$f; fi; fi; done" >/dev/null
  walls >"$EV/od2-archive-listing.txt"
  local mixed_gz mixed_plain
  mixed_gz=$(awk -v s="$start1" '/^[0-9A-F]{24}\.gz / { n = substr($1, 1, 24); if (n >= s) c++ } END { print c + 0 }' "$EV/od2-archive-listing.txt")
  mixed_plain=$(awk -v s="$start1" '/^[0-9A-F]{24} / { if (substr($1, 1, 24) >= s) c++ } END { print c + 0 }' "$EV/od2-archive-listing.txt")
  echo "od2.replay_window_segments gz=$mixed_gz plain=$mixed_plain (from base start $start1) target_time=$target"
  ck "OD2 the replay window really mixes both forms (>= 1 .gz and >= 1 plain)" bash -c "[ $mixed_gz -ge 1 ] && [ $mixed_plain -ge 1 ]"
  local rc
  ops /ops/restore-pitr.sh --target-time "$target" --drill >"$EV/od2-restore-pitr.log" 2>&1; rc=$?
  sed -n '1,40p' "$EV/od2-restore-pitr.log" | sed 's/^/  | /'
  ck "OD2 restore-pitr.sh --drill exits 0 (replay reached the target; witness rows 1,2,3 present, 4 absent)" test "$rc" = 0
  ck "OD2 log shows the paused-at-target verify" grep -q 'verify=PASS drill=1 result=ok' "$EV/od2-restore-pitr.log"

  # ---- OD3: retention. Negative control first: the pre-fix command on a synthetic mixed archive (24-hex names: TL 8 + LOG 8 + SEG 8).
  local P=0000000100000000000000 # 22 chars; + a 2-digit segment number = a 24-char WAL file name
  echo "info: OD3 negative control: legacy pg_archivecleanup (no -x .gz) on a synthetic mixed archive"
  local synth="set -e; d=/tmp/syn; rm -rf \$d; mkdir -p \$d; cd \$d
    for i in 01 02 03 04 05 06 07 08; do if ((10#\$i % 2)); then : >$P\$i.gz; else : >$P\$i; fi; done
    : >00000002.history; : >${P}03.00000028.backup; : >${P}07.00000028.backup"
  ops -c "$synth; pg_archivecleanup /tmp/syn ${P}06; ls /tmp/syn | LC_ALL=C sort | tr '\n' ' '" >"$TMP/old-clean.txt"
  echo "od3.negative_control_legacy_cleanup_leaves: $(cat "$TMP/old-clean.txt")"
  ck "OD3 negative control: the legacy command leaves the old .gz segments behind (unbounded growth)" grep -q "${P}01.gz" "$TMP/old-clean.txt"
  ops -c "$synth; pg_archivecleanup -b -x .gz /tmp/syn ${P}06; ls /tmp/syn | LC_ALL=C sort | tr '\n' ' '" >"$TMP/new-clean.txt"
  echo "od3.new_cleanup_leaves: $(cat "$TMP/new-clean.txt")"
  ck "OD3 pg_archivecleanup -b -x .gz leaves exactly: >= cutoff (both forms), the newer .backup, .history" \
    test "$(cat "$TMP/new-clean.txt")" = "${P}06 ${P}07.00000028.backup ${P}07.gz ${P}08 00000002.history "
  # Real basebackup.sh #2 (nothing newer than base #1's start may go) and #3 (keep = 2 drops base #1 and prunes everything
  # older than the START segment of the oldest kept base, #2).
  echo "info: OD3 base backups #2 and #3 (real basebackup.sh retention)"
  switch_name >/dev/null
  switch_name >/dev/null
  sleep 3
  ops /ops/basebackup.sh >"$EV/od3-basebackup2.log" 2>&1 || bad "OD3 basebackup.sh #2 failed (see $EV/od3-basebackup2.log)"
  switch_name >/dev/null
  switch_name >/dev/null
  sleep 3
  ops -c ": >/backup/wal/00000002.history" # a timeline history file the cleanup must never touch (name only)
  walls >"$TMP/before-prune.txt"
  ops /ops/basebackup.sh >"$EV/od3-basebackup3.log" 2>&1 || bad "OD3 basebackup.sh #3 failed (see $EV/od3-basebackup3.log)"
  local cut nbases
  cut=$(ops -c "tar -xzOf \$(ls -d /backup/base/20*T*Z | sort | head -n1)/base.tar.gz backup_label | sed -n 's/^START WAL LOCATION: .* (file \\([0-9A-F]\\{24\\}\\))\$/\\1/p'")
  nbases=$(ops -c 'ls /backup/base | grep -cE "^20[0-9]{6}T[0-9]{6}Z$"')
  walls >"$TMP/after-prune.txt"
  docker exec "$PG" bash -c 'ls /backup/wal | grep -E "\.backup$|\.history$"' >"$EV/od3-labels-after.txt"
  cp "$TMP/before-prune.txt" "$EV/od3-archive-before-prune.txt" && cp "$TMP/after-prune.txt" "$EV/od3-archive-after-prune.txt"
  local gz_below plain_below
  gz_below=$(awk -v c="$cut" '/^[0-9A-F]{24}\.gz / { if (substr($1, 1, 24) < c) n++ } END { print n + 0 }' "$TMP/before-prune.txt")
  plain_below=$(awk -v c="$cut" '/^[0-9A-F]{24} / { if (substr($1, 1, 24) < c) n++ } END { print n + 0 }' "$TMP/before-prune.txt")
  echo "od3.cutoff_segment=$cut bases_kept=$nbases before: gz_below_cutoff=$gz_below plain_below_cutoff=$plain_below files=$(wc -l <"$TMP/before-prune.txt" | tr -d ' ') after: files=$(wc -l <"$TMP/after-prune.txt" | tr -d ' ')"
  ck "OD3 two bases kept (LC_BASE_KEEP=2), the oldest was removed" test "$nbases" = 2
  ck "OD3 both forms were present below the cutoff before pruning (>= 1 .gz and >= 1 plain)" bash -c "[ $gz_below -ge 1 ] && [ $plain_below -ge 1 ]"
  ck "OD3 no segment (gz or plain) older than the oldest kept base's START segment is left" \
    bash -c "! awk -v c='$cut' '/^[0-9A-F]{24}(\\.gz)? / { if (substr(\$1, 1, 24) < c) f = 1 } END { exit !f }' '$TMP/after-prune.txt'"
  ck "OD3 nothing at or after the cutoff was touched (every such file present before is present after, same size)" \
    bash -c "awk -v c='$cut' '/^[0-9A-F]{24}(\\.gz)? / { if (substr(\$1, 1, 24) >= c) print }' '$TMP/before-prune.txt' | while read -r name size; do grep -qx \"\$name \$size\" '$TMP/after-prune.txt' || exit 1; done"
  ck "OD3 the timeline .history file survives" grep -qx '00000002.history' "$EV/od3-labels-after.txt"
  ck "OD3 the .backup label of the pruned base is gone (< cutoff), the kept bases' labels stay (>= 2)" \
    bash -c "! grep -q '^${start1}\\.' '$EV/od3-labels-after.txt' && [ \$(grep -c '\\.backup\$' '$EV/od3-labels-after.txt') -ge 2 ]"
  ck "OD3 archiver still healthy at the end (failed_count 0)" test "$(arch | cut -d'|' -f2)" = 0
  docker rm -f "$PG" >/dev/null 2>&1 || true
}

# ---------------------------------------------------------------------------------------------------------------- OD4/5
od_inner() { # alerts|disk — run the matching in-container block of this file in the pinned Linux image
  local which=$1 extra=()
  local W="$TMP/$which"
  mkdir -p "$W"
  if [[ "$which" == disk ]]; then extra=(--tmpfs /small:rw,size=8m); fi
  docker run --rm --pull=never --label "livecommerce.fixture=$RUN" --name "$RUN-$which" -v "$SRC:/repo:ro" -v "$W:/w" \
    -v "$ROOT/tests/deploy/ops-disk-guard.sh:/od.sh:ro" ${extra[@]+"${extra[@]}"} --entrypoint bash "$IMG" /od.sh "inner-$which" /w
}

# --------------------------------------------------------------------------------------------------------------- OD4b
od_smtp() {
  local D="$TMP/smtp" pw port mockpid i fn realcurl rc
  command -v python3 >/dev/null && command -v openssl >/dev/null && command -v curl >/dev/null || {
    echo "NOT_RUN: OD4b needs python3, openssl and curl on the host" >&2
    return
  }
  mkdir -p "$D/secrets" "$D/bin"
  openssl req -x509 -newkey rsa:2048 -nodes -keyout "$D/key.pem" -out "$D/crt.pem" -days 1 -subj /CN=127.0.0.1 \
    -addext subjectAltName=IP:127.0.0.1 >/dev/null 2>&1 || {
    echo "NOT_RUN: OD4b needs an openssl that supports -addext (3.x)" >&2
    return
  }
  cat >"$D/mock.py" <<'PY'
# Local mock SMTP relay: STARTTLS (self-signed cert), AUTH PLAIN/LOGIN against $D/expected_pw, MAIL/RCPT/DATA. Logs facts, never the password.
import os, socket, ssl, sys, base64
port_file, crt, key, out = sys.argv[1:5]
ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain(crt, key)
srv = socket.socket()
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(("127.0.0.1", 0))
srv.listen(8)
open(port_file, "w").write(str(srv.getsockname()[1]))
msgs = 0
def log(m):
    with open(os.path.join(out, "session.log"), "a") as f:
        f.write(m + "\n")
def readline(s):
    buf = b""
    while not buf.endswith(b"\r\n"):
        c = s.recv(1)
        if not c:
            raise EOFError
        buf += c
    return buf[:-2]
while True:
    conn, _ = srv.accept()
    s, tls, authed, mail_from, rcpt = conn, False, False, None, []
    try:
        s.sendall(b"220 mock ESMTP\r\n")
        while True:
            line = readline(s).decode("latin-1")
            cmd = line.upper()
            if cmd.startswith("EHLO") or cmd.startswith("HELO"):
                s.sendall(b"250-mock\r\n250-AUTH PLAIN LOGIN\r\n250 8BITMIME\r\n" if tls else b"250-mock\r\n250-STARTTLS\r\n250 8BITMIME\r\n")
            elif cmd == "STARTTLS":
                s.sendall(b"220 go ahead\r\n")
                s = ctx.wrap_socket(s, server_side=True)
                tls = True
                log("starttls")
            elif cmd.startswith("AUTH"):
                parts = line.split()
                expected = open(os.path.join(out, "expected_pw")).read().rstrip("\n")
                if parts[1].upper() == "PLAIN":
                    if len(parts) > 2:
                        blob = parts[2]
                    else:
                        s.sendall(b"334 \r\n")
                        blob = readline(s).decode()
                    _, user, pw = base64.b64decode(blob).decode().split("\0")
                else:
                    s.sendall(b"334 VXNlcm5hbWU6\r\n")
                    user = base64.b64decode(readline(s)).decode()
                    s.sendall(b"334 UGFzc3dvcmQ6\r\n")
                    pw = base64.b64decode(readline(s)).decode()
                authed = tls and user == "sender@fake.invalid" and pw == expected
                log("auth %s user=%s" % ("ok" if authed else "FAILED", user))
                s.sendall(b"235 ok\r\n" if authed else b"535 authentication failed\r\n")
            elif cmd.startswith("MAIL FROM"):
                if not authed:
                    s.sendall(b"530 authentication required\r\n")
                    continue
                mail_from = line.split(":", 1)[1].strip()
                s.sendall(b"250 ok\r\n")
            elif cmd.startswith("RCPT TO"):
                rcpt.append(line.split(":", 1)[1].strip())
                s.sendall(b"250 ok\r\n")
            elif cmd == "DATA":
                s.sendall(b"354 go\r\n")
                data = b""
                while not data.endswith(b"\r\n.\r\n"):
                    data += s.recv(4096)
                msgs += 1
                open(os.path.join(out, "msg.%d.eml" % msgs), "wb").write(data[:-5])
                log("data from=%s rcpt=%s tls=%s bytes=%d" % (mail_from, ",".join(rcpt), "yes" if tls else "no", len(data)))
                s.sendall(b"250 queued\r\n")
            elif cmd == "QUIT":
                s.sendall(b"221 bye\r\n")
                break
            else:
                s.sendall(b"250 ok\r\n")
    except (EOFError, ssl.SSLError, ConnectionError, OSError):
        pass
    finally:
        try:
            s.close()
        except OSError:
            pass
PY
  pw=$(secret)
  printf '%s\n' "$pw" >"$D/secrets/commerce_smtp_password"
  printf '%s\n' "$pw" >"$D/expected_pw"
  python3 "$D/mock.py" "$D/port" "$D/crt.pem" "$D/key.pem" "$D" >"$D/mock.out" 2>&1 &
  mockpid=$!
  for ((i = 0; i < 50; i++)); do [[ -s "$D/port" ]] && break; sleep 0.1; done
  port=$(cat "$D/port" 2>/dev/null)
  [[ -n "$port" ]] || { bad "OD4b the mock SMTP server did not start"; kill "$mockpid" 2>/dev/null; return; }
  # the function under test, verbatim from watchdog.sh (re_addr + alert_mail)
  fn=$(awk '/^re_addr=/ { f = 1 } f { print } f && /^}$/ { exit }' "$SRC/deploy/scripts/watchdog.sh")
  [[ "$fn" == *alert_mail* ]] || { bad "OD4b cannot extract alert_mail from watchdog.sh"; kill "$mockpid" 2>/dev/null; return; }
  { echo 'lc_warn() { echo "WARN $*" >&2; }'; echo "$fn"; echo 'alert_mail "[live-commerce] ALERT: W1 W2" "line one
line two"'; } >"$D/run.sh"
  realcurl=$(command -v curl)
  printf '#!/bin/bash\nprintf "%%s\\n" "$*" >>"%s/curl.argv"\nexec "%s" "$@"\n' "$D" "$realcurl" >"$D/bin/curl"
  chmod +x "$D/bin/curl"
  mail() { # [NAME=VALUE...] — alert_mail with the real curl (argv logged by the wrapper) against the mock; CA only when asked
    env -i PATH="$D/bin:$PATH" HOME="$TMP" LC_SMTP_HOST=127.0.0.1 LC_ALERT_SMTP_PORT="$port" LC_SMTP_USERNAME=sender@fake.invalid \
      LC_ALERT_EMAIL=owner@fake.invalid LC_SECRETS_DIR="$D/secrets" "$@" /bin/bash "$D/run.sh" >"$D/run.out" 2>&1
    rc=$?
  }
  mail CURL_CA_BUNDLE="$D/crt.pem"
  ck "OD4b real curl + mock relay: STARTTLS, AUTH and DATA accepted (exit 0)" test "$rc" = 0
  ck "OD4b the relay saw TLS, a good login, the right envelope" bash -c "grep -q '^starttls' '$D/session.log' && grep -q '^auth ok user=sender@fake.invalid' '$D/session.log' && grep -q '^data from=<sender@fake.invalid> rcpt=<owner@fake.invalid> tls=yes' '$D/session.log'"
  ck "OD4b the message is well formed: CRLF headers (From/To/Subject/Date/Message-ID/MIME) and the body lines" \
    bash -c "f='$D/msg.1.eml'; for h in 'From: sender@fake.invalid' 'To: owner@fake.invalid' 'Subject: \[live-commerce\] ALERT: W1 W2' 'Date: ' 'Message-ID: <lc-watchdog' 'MIME-Version: 1.0' 'line one' 'line two'; do grep -q \"^\$h\" \"\$f\" || exit 1; done; [ \"\$(grep -c \$'\\r\$' \"\$f\")\" -ge 8 ]"
  ck "OD4b the password is in no argv of the curl that authenticated with it" bash -c "! grep -qF '$pw' '$D/curl.argv' && grep -q -- '-K /dev/fd/' '$D/curl.argv'"
  ck "OD4b nothing the function printed contains the password" bash -c "! grep -qF '$pw' '$D/run.out'"
  local seen
  seen=$(wc -l <"$D/session.log" | tr -d ' ')
  mail # no CA bundle: the relay's certificate is self-signed, so a verifying curl must refuse it
  ck "OD4b an untrusted certificate is refused (exit 1, curl TLS error, no auth attempted)" bash -c "[ $rc = 1 ] && grep -q 'alert mail not sent (curl exit' '$D/run.out' && ! tail -n +$((seen + 1)) '$D/session.log' | grep -q '^auth'"
  local pw2
  pw2=$(secret)
  printf '%s\n' "$pw2" >"$D/secrets/commerce_smtp_password" # the relay still expects the first one: a rejected login
  mail CURL_CA_BUNDLE="$D/crt.pem"
  ck "OD4b a wrong password is refused by the relay (exit 1) and neither password is printed" bash -c "[ $rc = 1 ] && grep -q 'auth FAILED' '$D/session.log' && ! grep -qF '$pw' '$D/run.out' && ! grep -qF '$pw2' '$D/run.out' && ! grep -qF '$pw2' '$D/curl.argv'"
  # a password with quote, backslash, dollar and spaces must survive the curl config escaping
  local hard='p"w\x$HOME y#;'
  printf '%s\n' "$hard" >"$D/secrets/commerce_smtp_password"
  printf '%s\n' "$hard" >"$D/expected_pw"
  mail CURL_CA_BUNDLE="$D/crt.pem"
  ck "OD4b a password with quote, backslash, dollar and spaces authenticates (curl config escaping)" bash -c "[ $rc = 0 ] && tail -n 3 '$D/session.log' | grep -q '^auth ok'"
  mail CURL_CA_BUNDLE="$D/crt.pem" LC_ALERT_SMTP_PORT=1
  ck "OD4b a closed port is a clean failure (exit 1, warning, nothing leaked)" bash -c "[ $rc = 1 ] && grep -q 'alert mail not sent (curl exit' '$D/run.out' && ! grep -qF 'p\"w' '$D/run.out'"
  kill "$mockpid" 2>/dev/null
  wait "$mockpid" 2>/dev/null
  cp "$D/session.log" "$EV/od4b-mock-relay-session.log" 2>/dev/null || true
}

# ---------------------------------------------------------------------------------------------------------------- OD6
od_prune() {
  local pfx="${RUN}t" W="$TMP/prune" img tag real list
  mkdir -p "$W/state" "$W/bin"
  real=$(command -v docker)
  docker image inspect alpine:latest >/dev/null 2>&1 || {
    echo "NOT_RUN: OD6 needs a tiny local base image (alpine:latest) to retag as fake lc images" >&2
    return
  }
  cat >"$W/bin/docker" <<STUB
#!/usr/bin/env bash
# \`docker builder prune\` would trim the developer's REAL build cache: log it instead; everything else is real docker.
if [[ "\$1 \$2" == "builder prune" && " \$* " != *" --help "* ]]; then echo "\$*" >>"$W/builder.calls"; exit 0; fi
exec "$real" "\$@"
STUB
  chmod +x "$W/bin/docker"
  # One DISTINCT image per (name, tag), like real builds: with shared image ids `docker rmi name:tag` would only untag and an
  # in-use image could not be told apart. A committed empty container + a label gives each its own id without any network.
  docker create --name "$RUN-base" --label "livecommerce.fixture=$RUN" alpine:latest true >/dev/null
  for tag in a1 b2 c3 d4 e5; do for img in go admin storefront caddy; do docker commit --change "LABEL od=$img-$tag" "$RUN-base" "$pfx-$img:$tag" >/dev/null; done; done
  docker commit --change "LABEL od=other" "$RUN-base" "${pfx}x-go:a1" >/dev/null # a different prefix: must never be touched
  # history: a1 (oldest), b2 (PREVIOUS deployed), c3 (running); d4 was just built; e5 is an old build never deployed
  printf '%s\n' "2026-10-01T00:00:00Z	first	a1	100	op" "2026-10-02T00:00:00Z	upgrade	b2	105	op" "2026-10-03T00:00:00Z	upgrade	c3	105	op" >"$W/state/deployments.log"
  printf 'IMAGE_TAG=c3\nLC_IMAGE_PREFIX=%s\nLC_STATE_DIR=%s\n' "$pfx" "$W/state" >"$W/compose.env"
  prune() { (cd "$W" && env PATH="$W/bin:$PATH" LC_COMPOSE_ENV="$W/compose.env" LC_CONFIG_DIR="$W" LC_IMAGE_PREFIX="$pfx" LC_STATE_DIR="$W/state" bash "$SRC/deploy/scripts/prune-docker.sh" "$@" 2>&1); }
  tags_of() { docker image ls --format '{{.Tag}}' "$pfx-$1" | sort | tr '\n' ' '; }
  # dry run: prints, removes nothing
  prune --keep d4 --dry-run >"$TMP/prune-dry.txt"; local rc=$?
  ck "OD6 dry run: exit 0, names the removals, removes nothing" bash -c "[ $rc = 0 ] && grep -q 'would remove ${pfx}-go:e5' '$TMP/prune-dry.txt' && grep -q 'would remove ${pfx}-go:a1' '$TMP/prune-dry.txt' && [ \"\$(docker image ls --format '{{.Tag}}' '$pfx-go' | wc -l | tr -d ' ')\" = 5 ]"
  # a container (even stopped) holds go:a1: it must be reported as kept, never forced
  docker create --name "$RUN-holder" --label "livecommerce.fixture=$RUN" "$pfx-go:a1" true >/dev/null
  prune --keep d4 >"$TMP/prune-run.txt"; rc=$?
  cat "$TMP/prune-run.txt" | sed 's/^/  | /'
  ck "OD6 prune exits 0" test "$rc" = 0
  for img in go admin storefront caddy; do
    list=$(tags_of "$img")
    if [[ "$img" == go ]]; then
      ck "OD6 $img keeps new(d4) + running(c3) + previous(b2) and the in-use a1; e5 removed" test "$list" = "a1 b2 c3 d4 "
    else
      ck "OD6 $img keeps new(d4) + running(c3) + previous(b2); a1 and e5 removed" test "$list" = "b2 c3 d4 "
    fi
  done
  ck "OD6 an image of another prefix is untouched" test "$(docker image ls --format '{{.Tag}}' "${pfx}x-go")" = a1
  ck "OD6 the previous deployed tag (b2) survived for app-rollback" test "$(docker image inspect "$pfx-go:b2" "$pfx-admin:b2" "$pfx-storefront:b2" "$pfx-caddy:b2" 2>/dev/null | grep -c '"RepoTags"')" = 4
  ck "OD6 build cache trimmed to 3GB (docker builder prune -f --keep-storage|--reserved-space 3GB)" grep -Eq '^builder prune -f --(keep-storage|reserved-space) 3GB$' "$W/builder.calls"
  # no deployments.log (developer machine / never deployed): nothing is removed, no cache prune
  docker commit --change "LABEL od=z9" "$RUN-base" "$pfx-go:z9" >/dev/null
  mv "$W/state/deployments.log" "$W/state/deployments.log.off"
  : >"$W/builder.calls"
  prune --keep d4 >"$TMP/prune-nolog.txt"; rc=$?
  ck "OD6 without deployments.log: exit 0, nothing removed, no cache prune" bash -c "[ $rc = 0 ] && grep -q 'nothing to prune' '$TMP/prune-nolog.txt' && docker image inspect '$pfx-go:z9' >/dev/null 2>&1 && [ ! -s '$W/builder.calls' ]"
  # the wiring: build-images.sh calls it only after the build+report and honours LC_BUILD_PRUNE=0
  local a b
  a=$(grep -n '^  docker build ' "$SRC/deploy/scripts/build-images.sh" | head -n1 | cut -d: -f1)
  b=$(grep -n '^  "\$LC_SCRIPTS_DIR/prune-docker.sh"' "$SRC/deploy/scripts/build-images.sh" | head -n1 | cut -d: -f1)
  ck "OD6 build-images.sh runs prune-docker.sh after the build (line $b > $a), non-fatal, LC_BUILD_PRUNE=0 skips" \
    bash -c "[ -n '$a' ] && [ -n '$b' ] && [ '$b' -gt '$a' ] && grep -q 'LC_BUILD_PRUNE:-1' '$SRC/deploy/scripts/build-images.sh' && grep -q 'prune-docker.sh failed (non-fatal' '$SRC/deploy/scripts/build-images.sh'"
  docker rm -f "$RUN-holder" >/dev/null 2>&1 || true
}

case "$mode" in
pg) od_pg ;;
alerts)
  od_inner alerts || fails=$((fails + 1))
  od_smtp
  ;;
smtp) od_smtp ;;
disk) od_inner disk || fails=$((fails + 1)) ;;
prune) od_prune ;;
all)
  od_pg
  od_inner alerts || fails=$((fails + 1))
  od_smtp
  od_inner disk || fails=$((fails + 1))
  od_prune
  ;;
esac
if ((fails)); then
  printf 'ops-disk-guard (%s): FAIL (%s failing checks)\n' "$mode" "$fails"
  exit 1
fi
printf 'ops-disk-guard (%s): PASS\n' "$mode"
