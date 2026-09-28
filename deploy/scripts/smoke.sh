#!/usr/bin/env bash
# File: deploy/scripts/smoke.sh
# Purpose: automated acceptance of the deploy package (deploy-design §17).
#   static  S01-S06: script syntax (+shellcheck when installed), digest pins, compose config for
#           all profile sets (+ two-host override), lcentry vet/tests/coverage, Caddyfile
#           validate/fmt, ignore files. Needs no running containers.
#   full    static + S07-S43 on an ISOLATED project "lc-smoke-<run>" with temp config, temp
#           secrets and temp backup dir, *.localhost hosts on 127.0.0.1, identity=0 (no IdP),
#           buyer=1, buyer_payment=0, meta_webhook=0, studio=0. It touches ONLY its own
#           project and removes its containers/volumes/networks/temp dirs at the end.
#           Review-P1 regression cases (2026-09-28): S40 access-log redaction of credential query
#           parameters (Caddy), S41 superuser rotation without the password reaching the postgres
#           log, S42 PITR promote + cut-over (new timeline, rows == target, archiver healthy),
#           S43 compose.env IMAGE_TAG follows deploy.sh (plain `up -d` keeps the tag; watchdog W10).
# Usage: smoke.sh static | full
# Exit: 0 PASS, 1 FAIL, 3 BLOCKED (e.g. cmd/migrate missing, ports busy, docker missing, or a
#   REQUIRES_INTEGRATOR item observed at runtime: S29m = I8). result.json "not_run" names each one.
# Evidence: deploy/.evidence/<run_id>/result.json (fields per 架构.md line 992) + logs/ +
#   cases.tsv + commands.tsv. Logs are secret-scanned before the temp secrets are deleted.
# Runs as/in: developer/deploy host. `full` needs root (chown 999 for the backup dir) and free
#   ports LC_SMOKE_HTTP_PORT (80) / LC_SMOKE_HTTPS_PORT (443) on 127.0.0.1.
# Reads env: LC_SMOKE_HTTP_PORT, LC_SMOKE_HTTPS_PORT, LC_SMOKE_KEEP=1 (keep stack for debugging).
# Reads secrets: only the temp secrets it generates itself (never /etc/live-commerce).
# Used by: operators, test_worker/security_reviewer acceptance, CI (static).
# Depends on: bash, docker + compose, go (S04), python3, curl, openssl, node + repo Playwright
#   (S34 optional), every other deploy/scripts/*.sh.
# Status: DESIGN. static = runnable now; full = BLOCKED at S07 until cmd/migrate exists (I1), and
#   BLOCKED at S29m until the restore-stable media gate lands (I8). VERIFIED_LOCAL 2026-09-28 with
#   the I1 proposal: S08 (count regex), S21 (never checked Location), S34 (admin redirect leaked
#   the internal listener) and S39 (no-op rollback post-check) failed; all four are fixed here or
#   in compose.yml/admin.Dockerfile/deploy.sh.
# Change rules: never delete/relax a case to get green; a new deploy file needs a case here.
set -Eeuo pipefail
# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

mode=${1:-static}
[[ "$mode" == static || "$mode" == full ]] || lc_die "usage: smoke.sh static|full" 2
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$(od -An -N2 -tx2 /dev/urandom | tr -d ' ')"
EV="$LC_DEPLOY_DIR/.evidence/$run_id"
mkdir -p "$EV/logs"
started_at=$(lc_ts)
: >"$EV/cases.tsv"
: >"$EV/commands.tsv"
SMOKE_ROOT="" PROJECT="" full_started=0

rec() { # id status note
  printf '%s\t%s\t%s\n' "$1" "$2" "${3:-}" >>"$EV/cases.tsv"
  lc_log CASE "$1 $2 ${3:-}"
}
# runc ID CMD... — run a command, log stdout+stderr to logs/<ID>.log, record command + exit code.
runc() {
  local id=$1 rc=0
  shift
  "$@" >>"$EV/logs/$id.log" 2>&1 || rc=$?
  printf '%s\t%s\t%s\n' "$id" "$rc" "$*" >>"$EV/commands.tsv"
  return "$rc"
}
has() { command -v "$1" >/dev/null 2>&1; }

# ================================ static ==========================================================
static_cases() {
  local f bad=0 out
  # S01 syntax
  for f in "$LC_DEPLOY_DIR"/scripts/*.sh "$LC_DEPLOY_DIR"/postgres/*.sh "$LC_DEPLOY_DIR"/postgres/ops/*.sh; do
    runc S01 bash -n "$f" || bad=1
  done
  if ((bad)); then rec S01 FAIL "bash -n"; else rec S01 PASS "bash -n on all deploy scripts"; fi
  if has shellcheck; then
    if runc S01 shellcheck -S warning -x "$LC_DEPLOY_DIR"/scripts/*.sh "$LC_DEPLOY_DIR"/postgres/*.sh "$LC_DEPLOY_DIR"/postgres/ops/*.sh; then
      rec S01 PASS "shellcheck -S warning"
    else rec S01 FAIL "shellcheck findings (logs/S01.log)"; fi
  else
    rec S01 NOT_RUN "shellcheck not installed"
  fi
  # S02 pins
  if runc S02 "$LC_SCRIPTS_DIR/check-pins.sh"; then rec S02 PASS "digest pins"; else rec S02 FAIL "unpinned image (logs/S02.log)"; fi
  # S03 compose config
  if has docker && docker compose version >/dev/null 2>&1; then
    local cfg p ok=1
    cfg=$(mktemp -d)
    make_config "$cfg" static
    for p in "db,app,payments-sandbox,payments-live,meta,ops" "app,payments-sandbox" "db,ops" "app,meta"; do
      COMPOSE_PROFILES=$p runc S03 docker compose --project-directory "$LC_DEPLOY_DIR" --env-file "$cfg/compose.env" \
        -f "$LC_DEPLOY_DIR/compose.yml" config -q || ok=0
      COMPOSE_PROFILES=$p LC_PG_BIND_ADDR=127.0.0.1 runc S03 docker compose --project-directory "$LC_DEPLOY_DIR" \
        --env-file "$cfg/compose.env" -f "$LC_DEPLOY_DIR/compose.yml" -f "$LC_DEPLOY_DIR/compose.two-host-db.yml" config -q || ok=0
    done
    rm -rf "$cfg"
    if ((ok)); then rec S03 PASS "compose config (4 profile sets x 2 files)"; else rec S03 FAIL "compose config (logs/S03.log)"; fi
  else
    rec S03 NOT_RUN "docker compose not available"
  fi
  # S04 lcentry
  if has go; then
    if (cd "$LC_REPO_ROOT" && runc S04 go vet ./deploy/tools/... && runc S04 go test -count=1 -cover ./deploy/tools/...); then
      out=$(grep -o 'coverage: [0-9.]*%' "$EV/logs/S04.log" | tail -n1 | tr -dc '0-9.')
      if awk -v c="${out:-0}" 'BEGIN { exit !(c >= 90) }'; then rec S04 PASS "coverage=${out}%"; else rec S04 FAIL "coverage=${out:-?}% < 90%"; fi
    else
      rec S04 FAIL "go vet/test (logs/S04.log)"
    fi
  else
    rec S04 NOT_RUN "go not installed"
  fi
  # S05 Caddyfile
  if has docker; then
    local img
    img="${LC_IMAGE_PREFIX:-lc}-caddy:${SMOKE_TAG:-none}"
    docker image inspect "$img" >/dev/null 2>&1 ||
      img=$(sed -n 's/^ARG CADDY_IMAGE=//p' "$LC_DEPLOY_DIR/docker/caddy.Dockerfile")
    local cargs=(--rm --network none -e LC_ADMIN_HOST=admin.localhost -e LC_STORE_HOST=shop.localhost
      -e LC_API_HOST=api.localhost -e LC_HOOKS_HOST=hooks.localhost -e ACME_EMAIL=smoke@example.com
      -v "$LC_DEPLOY_DIR/caddy/Caddyfile:/etc/caddy/Caddyfile:ro")
    if runc S05 docker run "${cargs[@]}" "$img" caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile &&
      docker run "${cargs[@]}" "$img" caddy fmt /etc/caddy/Caddyfile >"$EV/logs/S05.fmt" 2>>"$EV/logs/S05.log" &&
      cmp -s "$EV/logs/S05.fmt" "$LC_DEPLOY_DIR/caddy/Caddyfile"; then
      rec S05 PASS "caddy validate + fmt (image ${img%%@*})"
    else
      diff "$LC_DEPLOY_DIR/caddy/Caddyfile" "$EV/logs/S05.fmt" >>"$EV/logs/S05.log" 2>&1 || true
      rec S05 FAIL "caddy validate/fmt (logs/S05.log)"
    fi
  else
    rec S05 NOT_RUN "docker not available"
  fi
  # S06 ignore files
  local pat miss=""
  for pat in '.git' '**/node_modules' '**/.next' 'deploy/.evidence' '**/.env' 'deploy/env/*.env' '**/*.pem' '**/*.key'; do
    grep -qxF -- "$pat" "$LC_REPO_ROOT/.dockerignore" || miss+=" .dockerignore:$pat"
  done
  for pat in '.evidence/' 'env/*.env' '!env/*.env.example'; do
    grep -qxF -- "$pat" "$LC_DEPLOY_DIR/.gitignore" || miss+=" deploy/.gitignore:$pat"
  done
  if [[ -z "$miss" ]]; then rec S06 PASS "ignore patterns"; else rec S06 FAIL "missing:$miss"; fi
}

# make_config DIR MODE — temp compose.env + env/*.env for smoke (placeholders, *.localhost).
make_config() {
  local dir=$1 kind=$2 svc
  mkdir -p "$dir/env" "$dir/secrets" "$dir/backup/dumps" "$dir/backup/base" "$dir/backup/wal" "$dir/state"
  for svc in api admin storefront payment-worker expiry-worker meta-worker caddy postgres; do
    cp "$LC_DEPLOY_DIR/env/$svc.env.example" "$dir/env/$svc.env"
  done
  sed -i -e 's/^COMMERCE_ACCOUNTS_ENABLED=.*/COMMERCE_ACCOUNTS_ENABLED=0/' \
    -e 's/^COMMERCE_BUYER_PAYMENT_ENABLED=.*/COMMERCE_BUYER_PAYMENT_ENABLED=0/' \
    -e 's/^COMMERCE_OIDC_CLIENT_ID=.*/COMMERCE_OIDC_CLIENT_ID=smoke-client/' "$dir/env/api.env"
  sed -i -e 's/^ACME_EMAIL=.*/ACME_EMAIL=smoke@example.com/' "$dir/env/caddy.env"
  cat >"$dir/compose.env" <<EOF
COMPOSE_PROJECT_NAME=${PROJECT:-lc-smoke-static}
COMPOSE_PROFILES=db,app,payments-sandbox,meta
IMAGE_TAG=${SMOKE_TAG:-smoke}
LC_IMAGE_PREFIX=${LC_IMAGE_PREFIX:-lc}
LC_ENVIRONMENT=smoke
LC_BIND_ADDR=127.0.0.1
LC_HTTP_PORT=${LC_SMOKE_HTTP_PORT:-80}
LC_HTTPS_PORT=${LC_SMOKE_HTTPS_PORT:-443}
LC_ADMIN_HOST=admin.localhost
LC_STORE_HOST=shop.localhost
LC_API_HOST=api.localhost
LC_HOOKS_HOST=hooks.localhost
LC_PUBLIC_IP=
LC_ENV_DIR=$dir/env
LC_SECRETS_DIR=$dir/secrets
LC_SECRETS_GID=10500
LC_BACKUP_DIR=$dir/backup
LC_STATE_DIR=$dir/state
LC_PG_HOST=postgres
LC_PG_SSLMODE=disable
LC_IDENTITY_ENABLED=0
LC_OIDC_ISSUER=
LC_ONBOARDING_ENABLED=0
LC_ONBOARDING_CURRENCIES=
LC_BUYER_ENABLED=1
LC_BUYER_SESSION_TTL_SECONDS=3600
LC_REQUIRE_MEDIA_GATE=0
LC_ALERT_WEBHOOK_URL=
EOF
  : "$kind"
}

# ================================ full ============================================================
ALL_FULL=(S07 S08 S09 S10 S11 S12 S13 S14 S15 S16 S17 S18 S19 S20 S21 S22 S23 S24 S25 S26 S27 S28 S29 S29m S30 S31 S32 S33 S34 S35 S36 S37 S38 S39 S40 S41 S42 S43)
block_rest() { # reason — mark every full case not yet recorded as BLOCKED
  local id
  for id in "${ALL_FULL[@]}"; do
    grep -q "^$id	" "$EV/cases.tsv" || rec "$id" BLOCKED "$1"
  done
}
# port_free PORT — try to bind 127.0.0.1:PORT (works without iproute2/ss).
port_free() {
  python3 -c 'import socket, sys
s = socket.socket()
s.bind(("127.0.0.1", int(sys.argv[1])))
s.close()' "$1" 2>/dev/null
}
CURL=(curl -sS --max-time 10)
edge() { # host path [extra curl args...] -> prints "code|content_type|size" ; body to $EV/logs/body
  local host=$1 path=$2
  shift 2
  "${CURL[@]}" --cacert "$CACERT" --resolve "$host:$HTTPS_PORT:127.0.0.1" -o "$EV/logs/body" -D "$EV/logs/headers" \
    -w '%{http_code}|%{content_type}|%{size_download}' "$@" "https://$host:$HTTPS_PORT$path" || echo "000||0"
}
# hdr NAME — value of a response header from the last edge() call ("" when absent; never fails).
hdr() { { grep -i "^$1:" "$EV/logs/headers" || true; } | head -n1 | cut -d: -f2- | tr -d '\r' | sed 's/^ //'; }

full_cases() {
  local rc
  has docker || { block_rest "docker not available" && return; }
  [[ $EUID == 0 ]] || { block_rest "full smoke needs root (backup dir owned by UID 999)" && return; }
  HTTP_PORT=${LC_SMOKE_HTTP_PORT:-80} HTTPS_PORT=${LC_SMOKE_HTTPS_PORT:-443}
  if ! port_free "$HTTP_PORT" || ! port_free "$HTTPS_PORT"; then block_rest "ports $HTTP_PORT/$HTTPS_PORT busy" && return; fi
  SMOKE_TAG="smoke-$(lc_git_tag)"
  PROJECT="lc-smoke-${run_id,,}"
  PROJECT=${PROJECT//[^a-z0-9_-]/-}

  # S07 build
  rc=0
  runc S07 "$LC_SCRIPTS_DIR/build-images.sh" --tag "$SMOKE_TAG" --evidence "$EV" || rc=$?
  if ((rc == 3)); then
    rec S07 BLOCKED "cmd/migrate missing (REQUIRES_INTEGRATOR I1)"
    block_rest "blocked by S07 (cmd/migrate missing)"
    return
  elif ((rc != 0)); then
    rec S07 FAIL "build-images.sh exit $rc (logs/S07.log)"
    block_rest "blocked by S07 failure"
    return
  fi
  rec S07 PASS "4 images tag=$SMOKE_TAG"

  # S08 image posture
  # lc-go must hold EXACTLY the binaries go.Dockerfile builds (ARG GO_CMDS) + lcentry. Compare
  # names, not a count: the tar listing also has the directory entry "app/bin/", which a `*`
  # regex counted as an 8th binary (VERIFIED_LOCAL F1); `\+` needs at least one name character.
  local p="${LC_IMAGE_PREFIX:-lc}" users bins want_bins caps cid
  users="$(for i in go admin storefront caddy; do docker image inspect -f '{{.Config.User}}' "$p-$i:$SMOKE_TAG"; done | tr '\n' ' ')"
  want_bins=$({
    sed -n 's/^ARG GO_CMDS="\(.*\)"$/\1/p' "$LC_DEPLOY_DIR/docker/go.Dockerfile" | tr ' ' '\n'
    echo lcentry
  } | sort | tr '\n' ' ')
  cid=$(docker create "$p-go:$SMOKE_TAG")
  bins=$(docker export "$cid" | tar -t | sed -n 's#^app/bin/\([a-z-]\+\)$#\1#p' | sort | tr '\n' ' ')
  docker rm "$cid" >/dev/null
  caps=$(docker run --rm --entrypoint getcap "$p-caddy:$SMOKE_TAG" /usr/bin/caddy 2>&1 || true)
  if [[ "$users" == "65532:65532 1000:1000 1000:1000 10001:10001 " && "$bins" == "$want_bins" && "$want_bins" == *migrate* && -z "$caps" ]]; then
    rec S08 PASS "users ok, lc-go binaries=[${bins% }], caddy file caps removed"
  else rec S08 FAIL "users=[$users] bins=[$bins] want=[$want_bins] caps=[$caps]"; fi

  # temp config
  SMOKE_ROOT=$(mktemp -d /tmp/lc-smoke.XXXXXX)
  chmod 0700 "$SMOKE_ROOT"
  make_config "$SMOKE_ROOT/config" full
  chmod 0750 "$SMOKE_ROOT/config/secrets"
  chgrp 10500 "$SMOKE_ROOT/config/secrets"
  chown -R 999:999 "$SMOKE_ROOT/config/backup"
  chmod -R 0700 "$SMOKE_ROOT/config/backup"
  export LC_CONFIG_DIR="$SMOKE_ROOT/config" LC_COMPOSE_ENV="$SMOKE_ROOT/config/compose.env"
  lc_load_env "$LC_COMPOSE_ENV"
  full_started=1

  # S09 secrets
  if runc S09 "$LC_SCRIPTS_DIR/secrets-init.sh"; then
    local before after modes
    before=$(cd "$LC_SECRETS_DIR" && sha256sum -- * | sha256sum)
    modes=$(stat -c '%a' "$LC_SECRETS_DIR"/* | sort -u | tr '\n' ' ')
    runc S09 "$LC_SCRIPTS_DIR/secrets-init.sh" || true
    after=$(cd "$LC_SECRETS_DIR" && sha256sum -- * | sha256sum)
    if [[ "$before" == "$after" && "$modes" == "440 " ]] && grep -q 'created=0 kept=36' "$EV/logs/S09.log"; then
      rec S09 PASS "36 files, mode 0440, idempotent"
    else rec S09 FAIL "modes=[$modes] checksum_stable=$([[ $before == "$after" ]] && echo yes || echo no)"; fi
  else rec S09 FAIL "secrets-init.sh (logs/S09.log)"; fi

  # S10 preflight positive + negatives
  if runc S10 "$LC_SCRIPTS_DIR/preflight.sh"; then rec S10 PASS "preflight positive"; else rec S10 FAIL "preflight positive (logs/S10.log)"; fi
  negative() { # id rule mutate-function
    local id=$1 rule=$2 copy
    copy=$(mktemp -d "$SMOKE_ROOT/neg.XXXXXX")
    cp -a "$SMOKE_ROOT/config/." "$copy/"
    "$3" "$copy"
    if (export LC_COMPOSE_ENV="$copy/compose.env" LC_ENV_DIR="$copy/env" LC_SECRETS_DIR="$copy/secrets" &&
      "$LC_SCRIPTS_DIR/preflight.sh" --skip-images >"$EV/logs/$id.log" 2>&1); then
      rec "$id" FAIL "preflight passed but $rule FAIL expected"
    elif grep -q "^$rule FAIL" "$EV/logs/$id.log"; then
      rec "$id" PASS "$rule FAIL as expected"
    else rec "$id" FAIL "failed without $rule"; fi
    rm -rf "$copy"
  }
  neg_a() { cp -f "$1/secrets/commerce_bff_key" "$1/secrets/commerce_buyer_bff_key"; }
  neg_b() { echo 'LISTEN_ADDR=0.0.0.0:8080' >>"$1/env/api.env"; }
  neg_c() { echo 'COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS=1' >>"$1/env/api.env"; }
  neg_d() { rm -f "$1/secrets/commerce_buyer_cookie_key"; }
  neg_e() { sed -i 's/^COMMERCE_STUDIO_ENABLED=.*/COMMERCE_STUDIO_ENABLED=1/' "$1/env/api.env"; }
  negative S10a P04 neg_a
  negative S10b P06 neg_b
  negative S10c P07 neg_c
  negative S10d P03 neg_d
  negative S10e P06 neg_e

  # S37 (+ S11-S16): the real first-deploy path
  if runc S37 "$LC_SCRIPTS_DIR/deploy.sh" --smoke first; then rec S37 PASS "deploy.sh first"; else
    rec S37 FAIL "deploy.sh first (logs/S37.log)"
    lc_compose logs --no-color >"$EV/logs/compose-after-S37.log" 2>&1 || true
    block_rest "blocked by S37 failure"
    return
  fi
  CACERT="$LC_STATE_DIR/smoke-caddy-root.crt"
  local v
  v=$(lc_psql <<<"SELECT current_setting('data_checksums') || '|' || current_setting('archive_mode');")
  if [[ "$v" == "on|on" ]]; then rec S11 PASS "data_checksums=on archive_mode=on"; else rec S11 FAIL "got $v"; fi

  local expected ledger
  expected=$(($(find "$LC_REPO_ROOT/migrations" -maxdepth 1 -name '[0-9][0-9][0-9][0-9]_*.sql' | wc -l) +
    $(find "$LC_REPO_ROOT/migrations/post_river" -maxdepth 1 -name '[0-9][0-9][0-9][0-9]_*.sql' | wc -l)))
  if runc S12 lc_compose run --rm -T migrate && runc S12 lc_compose run --rm -T migrate; then
    ledger=$(lc_ledger_count)
    if [[ "$ledger" == "$expected" ]]; then rec S12 PASS "migrate x2 exit 0, ledger=$ledger"; else rec S12 FAIL "ledger=$ledger expected=$expected"; fi
  else rec S12 FAIL "migrate re-run (logs/S12.log)"; fi

  if runc S13 lc_compose run --rm -T --no-deps provision-logins && runc S13 lc_compose run --rm -T --no-deps provision-logins &&
    [[ $(grep -c 'auth=ok' "$EV/logs/S13.log") == 24 ]] && ! grep -q DRIFT "$EV/logs/S13.log"; then
    rec S13 PASS "provision x2, 12 logins auth ok, matrix ok, readiness true"
  else rec S13 FAIL "provision-logins (logs/S13.log)"; fi

  if lc_compose exec -T postgres bash -c 'PGPASSWORD="$(< /run/secrets/pg_superuser_password)" psql -X -h postgres -U postgres -d live_commerce -c "SELECT 1"' \
    >"$EV/logs/S14.log" 2>&1; then
    rec S14 FAIL "superuser TCP login succeeded"
  elif grep -q 'pg_hba.conf rejects' "$EV/logs/S14.log"; then rec S14 PASS "superuser over TCP rejected by pg_hba"; else rec S14 FAIL "unexpected error (logs/S14.log)"; fi

  local s st bad=""
  for s in api admin storefront caddy postgres; do
    st=$(docker inspect -f '{{.State.Health.Status}}' "$(lc_compose ps -q "$s")" 2>/dev/null || echo none)
    [[ "$st" == healthy ]] || bad+=" $s=$st"
  done
  runc S15 lc_compose exec -T api /app/bin/lcentry probe http://127.0.0.1:8080/readyz || bad+=" api-probe"
  if [[ -z "$bad" ]]; then rec S15 PASS "api/admin/storefront/caddy/postgres healthy"; else rec S15 FAIL "$bad"; fi

  sleep 30
  bad=""
  for s in expiry-worker:expiry_worker_ready payment-worker-sandbox:payment_worker_ready meta-worker:meta_worker_ready; do
    grep -q "${s#*:}" < <(lc_compose logs --no-log-prefix "${s%%:*}" 2>&1) || bad+=" ${s%%:*}:no-token"
    st=$(docker inspect -f '{{.RestartCount}}' "$(lc_compose ps -q "${s%%:*}")")
    [[ "$st" == 0 ]] || bad+=" ${s%%:*}:restarts=$st"
  done
  if [[ -z "$bad" ]]; then rec S16 PASS "worker ready tokens, 0 restarts after 30 s"; else rec S16 FAIL "$bad"; fi

  # S17-S24 edge
  local r
  r=$(edge api.localhost /healthz)
  if [[ "${r%%|*}" == 200 ]] && grep -q '"status":"ok"' "$EV/logs/body"; then rec S17 PASS "api /healthz 200 via TLS (Caddy local CA)"; else rec S17 FAIL "got $r"; fi
  local r1 r2
  r1=$(edge api.localhost /readyz)
  r2=$(edge api.localhost /v1/admin/stores)
  if [[ "$r1" == "404||0" && "$r2" == "404||0" ]]; then rec S18 PASS "api host default-deny"; else rec S18 FAIL "readyz=$r1 stores=$r2"; fi
  r=$(edge hooks.localhost /v1/meta/webhooks/1/page)
  r1=$(edge hooks.localhost /payuni/notify)
  r2=$(edge hooks.localhost /)
  if [[ "${r%%|*}" == 404 && -n "$(cut -d'|' -f2 <<<"$r")" && "$r1" == "404||0" && "$r2" == "404||0" ]]; then
    rec S19 PASS "meta webhook route reaches Go API ($(cut -d'|' -f2 <<<"$r")); notify and / 404 at Caddy"
  else rec S19 FAIL "webhook=$r notify=$r1 root=$r2"; fi
  r=$(edge shop.localhost /payment/return)
  if [[ "${r%%|*}" == 200 ]] && grep -q 'data-testid="payment-return"' "$EV/logs/body" && [[ "$(hdr content-security-policy)" == *"default-src 'none'"* ]]; then
    rec S20 PASS "storefront /payment/return 200 + CSP"
    if [[ "$(hdr strict-transport-security)" == max-age=* && -z "$(hdr server)" ]]; then rec S23 PASS "HSTS present, Server header absent"; else rec S23 FAIL "hsts=[$(hdr strict-transport-security)] server=[$(hdr server)]"; fi
  else
    rec S20 FAIL "got $r"
    rec S23 FAIL "not evaluated (S20 failed)"
  fi
  # S21 admin "/": a locale redirect must stay on the PUBLIC origin (relative Location, or absolute
  # on https://admin.localhost[:port]/), and its target must render 200 with X-Frame-Options DENY.
  # "Any status < 500" was not enough: the redirect leaked https://localhost:3100/ (the internal
  # listener) and the browser failed (VERIFIED_LOCAL F2; fixed by HOSTNAME=localhost in compose.yml).
  local code loc xfo target="" admin_origin="https://admin.localhost"
  ((HTTPS_PORT == 443)) || admin_origin+=":$HTTPS_PORT"
  r=$(edge admin.localhost /)
  code=${r%%|*} loc=$(hdr location) xfo=$(hdr x-frame-options)
  case "$code" in
  200) target=/ ;;
  301 | 302 | 303 | 307 | 308)
    if [[ "$loc" == /* && "$loc" != //* ]]; then
      target=$loc
    elif [[ "$loc" == "$admin_origin"/* ]]; then target=${loc#"$admin_origin"}; fi
    ;;
  esac
  r1="not-fetched"
  [[ -z "$target" ]] || r1=$(edge admin.localhost "$target")
  if [[ -n "$target" && "$xfo" == DENY && "${r1%%|*}" == 200 && "$(hdr x-frame-options)" == DENY ]]; then
    rec S21 PASS "admin / $code -> ${loc:-/} -> 200, X-Frame-Options DENY"
  else rec S21 FAIL "admin / $code location=[$loc] (must be relative or $admin_origin/...) xfo=[$xfo] target=[$r1]"; fi
  r1=$("${CURL[@]}" -o /dev/null -w '%{http_code} %{redirect_url}' --resolve "shop.localhost:$HTTP_PORT:127.0.0.1" "http://shop.localhost:$HTTP_PORT/" || true)
  r2=$("${CURL[@]}" -o "$EV/logs/body" -D "$EV/logs/headers" -w '%{http_code}|%{size_download}' -H 'Host: unknown.example' "http://127.0.0.1:$HTTP_PORT/" || true)
  if [[ "$r1" == "308 https://shop.localhost"* && "${r2#*|}" == 0 && -z "$(hdr x-frame-options)" ]]; then
    rec S22 PASS "http->https 308; unknown Host not proxied (${r2%%|*}, empty)"
  else rec S22 FAIL "redirect=[$r1] unknown=[$r2]"; fi
  # S40 (review P1) access-log redaction: Caddy logs the full request URI, so a Meta
  # hub.verify_token (owner secret commerce_meta_apps_json) or an OIDC code/state on the admin
  # callback would sit in the json-file logs and diagnostics bundles. Random canaries go through
  # the hooks and admin hosts (query string, Referer, and the http->https redirect whose Location
  # echoes the query); none may appear in ANY container log, and the REDACTED forms must be there
  # (proves the lines were written, so 0 hits is meaningful).
  local canary hits
  canary="lcCanary$(od -An -N8 -tx1 /dev/urandom | tr -d ' \n')"
  edge hooks.localhost "/v1/meta/webhooks/1/page?hub.mode=subscribe&hub.challenge=42&hub.verify_token=${canary}v" >/dev/null
  edge admin.localhost "/api/auth/callback?code=${canary}c&state=${canary}s" \
    -H "Referer: https://admin.localhost/api/auth/callback?code=${canary}r&state=${canary}q" >/dev/null
  "${CURL[@]}" -o /dev/null --resolve "admin.localhost:$HTTP_PORT:127.0.0.1" \
    "http://admin.localhost:$HTTP_PORT/api/auth/callback?code=${canary}l&state=${canary}m" >/dev/null 2>&1 || true
  sleep 2
  lc_compose logs --no-color >"$EV/logs/S40-compose.log" 2>&1 || true
  hits=$({ grep -c -- "$canary" "$EV/logs/S40-compose.log" || true; } | tail -n1)
  if [[ "$hits" == 0 ]] && grep -q 'hub.challenge=42&hub.mode=subscribe&hub.verify_token=REDACTED' "$EV/logs/S40-compose.log" &&
    grep -q '/api/auth/callback?code=REDACTED&state=REDACTED' "$EV/logs/S40-compose.log" &&
    grep -q '"Referer":\["https://admin.localhost/api/auth/callback?code=REDACTED&state=REDACTED"\]' "$EV/logs/S40-compose.log" &&
    grep -q '"Location":\["https://admin.localhost[^"]*/api/auth/callback?code=REDACTED&state=REDACTED"\]' "$EV/logs/S40-compose.log"; then
    rec S40 PASS "canary verify_token/code/state absent from all logs; uri, Referer and redirect Location logged as REDACTED"
  else rec S40 FAIL "canary hits=$hits or REDACTED form missing (logs/S40-compose.log)"; fi
  runc S24 lc_compose stop api || true
  r=$(edge api.localhost /healthz)
  local retry
  retry=$(hdr retry-after)
  # `up -d --no-deps`, not `start`: start would also try to (re)start the one-shot dependencies.
  runc S24 lc_compose up -d --no-deps api || true
  for ((i = 0; i < 30; i++)); do
    [[ "$(docker inspect -f '{{.State.Health.Status}}' "$(lc_compose ps -q api)")" == healthy ]] && break
    sleep 2
  done
  r1=$(edge api.localhost /healthz)
  if [[ "${r%%|*}" == 503 && "$retry" == 60 && "${r1%%|*}" == 200 ]]; then rec S24 PASS "api down -> 503 Retry-After 60; back -> 200"; else rec S24 FAIL "down=$r retry=$retry up=$r1"; fi

  # S25 hardening
  bad=""
  local c
  while IFS= read -r c; do
    v=$(docker inspect -f '{{.Name}} ro={{.HostConfig.ReadonlyRootfs}} cap={{.HostConfig.CapDrop}} sec={{.HostConfig.SecurityOpt}} mem={{.HostConfig.Memory}} pids={{.HostConfig.PidsLimit}} log={{index .HostConfig.LogConfig.Config "max-size"}}/{{index .HostConfig.LogConfig.Config "max-file"}} user={{.Config.User}}' "$c")
    echo "$v" >>"$EV/logs/S25.log"
    [[ "$v" == *" ro=true "* && "$v" == *"cap=[ALL]"* && "$v" == *"no-new-privileges:true"* && "$v" != *" mem=0 "* &&
      "$v" != *" pids=0 "* && "$v" != *"pids=<nil>"* && "$v" == *" log=20m/5 "* && "$v" != *"user= "* && "$v" != *" user=0"* && "$v" != *"user=root"* ]] || bad+=" ${v%% *}"
  done < <(lc_compose ps -q)
  if [[ -z "$bad" ]]; then rec S25 PASS "all running containers hardened"; else rec S25 FAIL "not hardened:$bad"; fi

  # S26 / S27 secret leakage
  lc_compose ps -q | xargs docker inspect >"$EV/logs/S26-inspect.json" 2>&1 || true
  if lc_secret_scan "$EV/logs/S26-inspect.json" 2>>"$EV/logs/S26.log"; then rec S26 PASS "no secret value in docker inspect"; else rec S26 FAIL "secret in docker inspect (logs/S26.log)"; fi
  lc_compose logs --no-color >"$EV/logs/S27-compose.log" 2>&1 || true
  if lc_secret_scan "$EV/logs/S27-compose.log" 2>>"$EV/logs/S27.log"; then rec S27 PASS "no secret value in compose/PG logs"; else rec S27 FAIL "secret in logs (logs/S27.log)"; fi

  # S28-S31 backups
  local dump
  if runc S28 "$LC_SCRIPTS_DIR/pg-ops.sh" backup --tag smoke; then
    dump=$(find "$LC_BACKUP_DIR/dumps" -mindepth 1 -maxdepth 1 -type d -name '20*_smoke' | sort | tail -n1)
    if [[ -n "$dump" ]] && (cd "$dump" && sha256sum --quiet -c SHA256SUMS) &&
      runc S28 lc_compose_with_ops run --rm --no-deps -T pg-ops -c "pg_restore --list /backup/dumps/${dump##*/}/db.dump >/dev/null"; then
      rec S28 PASS "dump ${dump##*/} checksums ok, pg_restore --list ok"
    else rec S28 FAIL "dump verification (logs/S28.log)"; fi
  else rec S28 FAIL "pg-ops backup (logs/S28.log)"; fi
  if [[ -n "${dump:-}" ]] && runc S29 "$LC_SCRIPTS_DIR/pg-ops.sh" restore-dump "${dump##*/}" --drop-after-verify &&
    grep -q 'verify.result=PASS' "$EV/logs/S29.log"; then
    rec S29 PASS "restore into new DB, verify.sql PASS, dropped"
  else rec S29 FAIL "restore-dump (logs/S29.log)"; fi
  # S29m (I8 made visible): verify.sql only REQUIRES the media gate with LC_REQUIRE_MEDIA_GATE=1
  # (default 0: studio/media are not deployable in this release), so S29 passes although a logical
  # restore turns live.media_plan_ready() t -> f. Record that separately and honestly instead of
  # hiding it inside S29: BLOCKED (REQUIRES_INTEGRATOR I8) until migrations/ ship a restore-stable
  # gate; PASS once the restored value matches the live one.
  local live_media restored_media
  live_media=$(lc_psql <<<"SELECT live.media_plan_ready();" 2>>"$EV/logs/S29.log" || true)
  restored_media=$({ grep -o 'media_plan=[tf]' "$EV/logs/S29.log" || true; } | tail -n1 | cut -d= -f2)
  if [[ "$live_media" == t && "$restored_media" == t ]]; then
    rec S29m PASS "media gate live=t restored=t (I8 fixed: make LC_REQUIRE_MEDIA_GATE=1 the default)"
  elif [[ "$live_media" == t && "$restored_media" == f ]]; then
    rec S29m BLOCKED "REQUIRES_INTEGRATOR I8: logical restore turns live.media_plan_ready() t->f (not required while LC_REQUIRE_MEDIA_GATE=0)"
  else rec S29m FAIL "media gate live=[${live_media}] restored=[${restored_media}] (logs/S29.log)"; fi
  local a0 a1 f1
  a0=$(lc_psql <<<"SELECT archived_count FROM pg_stat_archiver;")
  lc_psql <<<"SELECT pg_logical_emit_message(false, 'lc-smoke', 'wal'); SELECT pg_switch_wal();" >/dev/null
  for ((i = 0; i < 30; i++)); do
    a1=$(lc_psql <<<"SELECT archived_count FROM pg_stat_archiver;")
    ((a1 > a0)) && break
    sleep 1
  done
  f1=$(lc_psql <<<"SELECT failed_count FROM pg_stat_archiver;")
  if ((a1 > a0 && f1 == 0)) && runc S30 "$LC_SCRIPTS_DIR/pg-ops.sh" basebackup; then
    rec S30 PASS "WAL archived ($a0->$a1, failed=0); basebackup + pg_verifybackup ok"
  else rec S30 FAIL "archived $a0->${a1:-?} failed=${f1:-?} (logs/S30.log)"; fi
  local target out
  sleep 12
  target=$(lc_psql <<<"SELECT now();")
  sleep 2
  lc_psql <<<"SELECT txid_current();" >/dev/null
  a0=$(lc_psql <<<"SELECT archived_count FROM pg_stat_archiver;")
  lc_psql <<<"SELECT pg_switch_wal();" >/dev/null
  for ((i = 0; i < 30; i++)); do
    a1=$(lc_psql <<<"SELECT archived_count FROM pg_stat_archiver;")
    ((a1 > a0)) && break
    sleep 1
  done
  if runc S31 "$LC_SCRIPTS_DIR/pg-ops.sh" restore-pitr --target-time "$target" --drill && grep -q 'verify.result=PASS' "$EV/logs/S31.log"; then
    out=$(grep -o 'pitr_duration_seconds=[0-9]*' "$EV/logs/S31.log" | tail -n1)
    out+=" $({ grep -o 'media_plan=[tf]' "$EV/logs/S31.log" || true; } | tail -n1)"
    rec S31 PASS "PITR drill paused at target, verify PASS, $out (RTO sample; physical restore keeps the media gate)"
  else rec S31 FAIL "restore-pitr drill (logs/S31.log)"; fi

  # S41 (review P1) superuser rotation: the live server runs log_statement='ddl', so a bare
  # ALTER ROLE ... PASSWORD would be logged in cleartext. pg-ops.sh rotate-superuser must change
  # the value (in place, same inode), keep the file mode, reject the old password, leave NO old or
  # new value in the postgres log, and everything that uses the superuser must keep working:
  # lc_psql (running container's mount), migrate (re-derived dsn_migrate_owner). A DDL canary
  # proves statement logging is live, so a clean scan is meaningful. Later cases (S35 watchdog,
  # S36 diagnostics scan, S38 backup/migrate/provision, S42 PITR) all run on the rotated password.
  local old_pw old_sum old_ino ddl_canary pglog="$EV/logs/S41-postgres.log" why41=""
  old_pw=$(<"$LC_SECRETS_DIR/pg_superuser_password")
  old_sum=$(sha256sum <"$LC_SECRETS_DIR/pg_superuser_password")
  old_ino=$(stat -c '%i %a' "$LC_SECRETS_DIR/pg_superuser_password")
  ddl_canary="lc_smoke_ddl_canary_$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
  if ! runc S41 "$LC_SCRIPTS_DIR/pg-ops.sh" rotate-superuser; then
    why41="rotate-superuser failed"
  elif [[ "$(sha256sum <"$LC_SECRETS_DIR/pg_superuser_password")" == "$old_sum" ]]; then
    why41="secret file unchanged"
  elif [[ "$(stat -c '%i %a' "$LC_SECRETS_DIR/pg_superuser_password")" != "$old_ino" ]]; then
    why41="secret file inode/mode changed (running container would keep the old value)"
  elif ! grep -q 'old_password=rejected result=ok' "$EV/logs/S41.log"; then
    why41="old password not proven rejected"
  elif [[ "$(lc_psql <<<"CREATE TEMP TABLE $ddl_canary (x int); SELECT 1;" 2>>"$EV/logs/S41.log")" != 1 ]]; then
    why41="lc_psql fails after rotation"
  elif ! runc S41 lc_compose run --rm -T migrate; then
    why41="migrate fails after rotation (dsn_migrate_owner not re-derived?)"
  else
    lc_compose logs --no-color postgres >"$pglog" 2>&1 || true
    if ! grep -q "$ddl_canary" "$pglog"; then
      why41="DDL canary not in the postgres log: statement logging not live, scan would be vacuous"
    elif ! lc_secret_scan "$pglog" 2>>"$EV/logs/S41.log"; then
      why41="a secret value (new superuser password?) is in the postgres log"
    elif grep -qFf <(printf '%s\n' "$old_pw") "$pglog"; then
      why41="the old superuser password is in the postgres log"
    fi
  fi
  unset old_pw
  if [[ -z "$why41" ]]; then
    rec S41 PASS "rotated in place (inode/mode kept), old rejected, lc_psql + migrate ok, DDL logging live, no old/new value in postgres log"
  else rec S41 FAIL "$why41 (logs/S41.log)"; fi

  # S34 browser (optional)
  if has node && (cd "$LC_REPO_ROOT" && node -e "import('@playwright/test')" >/dev/null 2>&1); then
    rc=0
    SMOKE_HTTPS_PORT=$HTTPS_PORT SMOKE_EVIDENCE_DIR="$EV" runc S34 node "$LC_SCRIPTS_DIR/smoke-browser.mjs" || rc=$?
    if ((rc == 0)); then rec S34 PASS "Chromium through the edge (screenshots in evidence)"; elif ((rc == 3)); then rec S34 NOT_RUN "Chromium not installed"; else rec S34 FAIL "smoke-browser.mjs (logs/S34.log)"; fi
  else rec S34 NOT_RUN "node/@playwright/test not available"; fi

  # S35 watchdog
  if LC_TLS_MIN_DAYS=0 runc S35 "$LC_SCRIPTS_DIR/watchdog.sh"; then
    runc S35 lc_compose stop expiry-worker || true
    if LC_TLS_MIN_DAYS=0 runc S35 "$LC_SCRIPTS_DIR/watchdog.sh"; then rec S35 FAIL "watchdog passed with expiry-worker stopped"; else
      runc S35 lc_compose up -d --no-deps expiry-worker || true
      sleep 5
      if grep -q '^W1 FAIL expiry-worker' "$EV/logs/S35.log" && LC_TLS_MIN_DAYS=0 runc S35 "$LC_SCRIPTS_DIR/watchdog.sh"; then
        rec S35 PASS "healthy=0, stopped worker -> W1, recovered=0"
      else rec S35 FAIL "watchdog negative/recovery (logs/S35.log)"; fi
    fi
  else rec S35 FAIL "watchdog on healthy stack (logs/S35.log)"; fi

  # S36 diagnostics
  mkdir -p "$SMOKE_ROOT/diag"
  if LC_DIAG_DIR="$SMOKE_ROOT/diag" runc S36 "$LC_SCRIPTS_DIR/collect-diagnostics.sh" --since 1h &&
    compgen -G "$SMOKE_ROOT/diag/lc-diag-*.tar.gz" >/dev/null; then
    rec S36 PASS "bundle built, secret scan clean"
  else rec S36 FAIL "collect-diagnostics (logs/S36.log)"; fi

  # S38 upgrade to the same tag with a 503 window observed
  local poll_pid codes
  (for ((i = 0; i < 240; i++)); do
    edge api.localhost /healthz | cut -d'|' -f1
    sleep 0.5
  done >"$EV/logs/S38-codes.txt") &
  poll_pid=$!
  if runc S38 "$LC_SCRIPTS_DIR/deploy.sh" --smoke upgrade "$SMOKE_TAG"; then
    sleep 2
    kill "$poll_pid" 2>/dev/null || true
    wait "$poll_pid" 2>/dev/null || true
    codes=$(sort -u "$EV/logs/S38-codes.txt" | tr '\n' ' ')
    if [[ " $codes" == *" 503 "* && "$(tail -n1 "$EV/logs/S38-codes.txt")" == 200 ]] && grep -q 'pre-upgrade' "$EV/logs/S38.log"; then
      rec S38 PASS "backup taken, 503 window observed, migrate no-op, healthy again (codes: $codes)"
    else rec S38 FAIL "codes=[$codes]"; fi
  else
    kill "$poll_pid" 2>/dev/null || true
    rec S38 FAIL "deploy.sh upgrade (logs/S38.log)"
  fi

  # S39 app-rollback, all four branches:
  #   a) to the tag that is already running: `up -d` is a no-op, post-checks must accept the ready
  #      tokens of the current container lifetimes (VERIFIED_LOCAL F3 hung here for 60 s);
  #   b) to a genuinely different tag ("<SMOKE_TAG>-rb", a retag of the same images recorded with
  #      the current ledger): every lc-* container is recreated and must run the -rb tag;
  #   c) back to SMOKE_TAG (recreate again);
  #   d) to a tag recorded with a different ledger count: REFUSED ("forward-fix only").
  # The -rb tags are removed afterwards (they only name existing images; nothing is rebuilt).
  # S43 (review P1) rides on b) and c): deploy.sh must write the tag it brings up into compose.env,
  # because every other Compose entry point (runbook `dc up`/`dc run migrate`, §8 domain change, a
  # re-run of `first`) takes IMAGE_TAG from that file. While the stack runs the -rb tag:
  #   compose.env says -rb; a PLAIN `up -d` with no IMAGE_TAG in the environment (what an operator's
  #   `dc up -d` does) keeps every lc-* service on -rb; watchdog W10 PASSes, and FAILs against a
  #   copy of compose.env naming another tag. After c) compose.env is back on SMOKE_TAG, and the
  #   refused d) leaves it untouched.
  local rb_tag="${SMOKE_TAG}-rb" why="" why43=""
  env_tag() { lc_env_file_get "$LC_COMPOSE_ENV" IMAGE_TAG || true; }
  s43_on_rb() {
    local s img w10copy
    [[ "$(env_tag)" == "$rb_tag" ]] || { why43="b) compose.env IMAGE_TAG=$(env_tag) after app-rollback $rb_tag" && return; }
    (unset IMAGE_TAG && runc S43 lc_compose up -d) || { why43="plain up -d failed" && return; }
    for s in api admin storefront caddy expiry-worker payment-worker-sandbox meta-worker; do
      img=$(docker inspect -f '{{.Config.Image}}' "$(lc_compose ps -q "$s")" 2>/dev/null || echo "<no container>")
      [[ "$img" == *":$rb_tag" ]] || { why43="plain up -d moved $s to $img" && return; }
    done
    (unset IMAGE_TAG && LC_TLS_MIN_DAYS=0 runc S43 "$LC_SCRIPTS_DIR/watchdog.sh") || true
    grep -q "^W10 PASS .*IMAGE_TAG=$rb_tag" "$EV/logs/S43.log" || { why43="watchdog W10 did not PASS on $rb_tag" && return; }
    w10copy=$(mktemp "$SMOKE_ROOT/w10.XXXXXX")
    cp -p "$LC_COMPOSE_ENV" "$w10copy"
    lc_env_file_set "$w10copy" IMAGE_TAG "$SMOKE_TAG"
    if (unset IMAGE_TAG && LC_COMPOSE_ENV=$w10copy LC_TLS_MIN_DAYS=0 runc S43 "$LC_SCRIPTS_DIR/watchdog.sh"); then
      why43="watchdog passed although compose.env names $SMOKE_TAG and containers run $rb_tag"
    elif ! grep -q "^W10 FAIL api runs tag $rb_tag but compose.env IMAGE_TAG=$SMOKE_TAG" "$EV/logs/S43.log"; then
      why43="W10 drift message missing"
    fi
    rm -f "$w10copy"
  }
  # s39_steps — a) .. d) in order; the first failing step sets `why` and stops the sequence.
  s39_steps() {
    runc S39 "$LC_SCRIPTS_DIR/deploy.sh" --smoke app-rollback "$SMOKE_TAG" ||
      { why="a) no-op rollback to the running tag failed" && return; }
    runc S39 "$LC_SCRIPTS_DIR/deploy.sh" --smoke app-rollback "$rb_tag" ||
      { why="b) rollback to a different tag failed" && return; }
    [[ "$(docker inspect -f '{{.Config.Image}}' "$(lc_compose ps -q api)" 2>/dev/null || true)" == "$p-go:$rb_tag" ]] ||
      { why="b) api was not recreated on $rb_tag" && return; }
    s43_on_rb # verdict in why43; S39 continues either way
    runc S39 "$LC_SCRIPTS_DIR/deploy.sh" --smoke app-rollback "$SMOKE_TAG" ||
      { why="c) rollback back to $SMOKE_TAG failed" && return; }
    if runc S39 "$LC_SCRIPTS_DIR/deploy.sh" --smoke app-rollback smoke-fabricated; then
      why="d) rollback with a changed ledger was allowed"
      return
    fi
    grep -q REFUSED "$EV/logs/S39.log" || why="d) refusal message missing"
  }
  for i in go admin storefront caddy; do docker tag "$p-$i:$SMOKE_TAG" "$p-$i:$rb_tag"; done
  printf '%s\tupgrade\t%s\t%s\t%s\n' "$(lc_ts)" "$rb_tag" "$(lc_ledger_count)" smoke >>"$LC_STATE_DIR/deployments.log"
  printf '%s\tupgrade\t%s\t%s\t%s\n' "$(lc_ts)" smoke-fabricated 1 smoke >>"$LC_STATE_DIR/deployments.log"
  s39_steps
  if [[ -z "$why" && -z "$why43" && "$(env_tag)" != "$SMOKE_TAG" ]]; then
    why43="c)/d) compose.env IMAGE_TAG=$(env_tag), expected $SMOKE_TAG"
  fi
  for i in go admin storefront caddy; do docker image rm "$p-$i:$rb_tag" >/dev/null 2>&1 || true; done
  if [[ -z "$why" ]]; then
    rec S39 PASS "rollback: no-op to running tag, to a different tag (recreated), back, refused on changed ledger"
  else rec S39 FAIL "$why (logs/S39.log)"; fi
  if [[ -n "$why" ]]; then
    rec S43 FAIL "not evaluated: S39 $why"
  elif [[ -z "$why43" ]]; then
    rec S43 PASS "compose.env follows app-rollback (-rb, back); plain up -d kept -rb; W10 PASS, and FAIL on drift"
  else rec S43 FAIL "$why43 (logs/S43.log)"; fi

  # S42 (review P1) PITR promotion + cut-over, backup-restore.md §6, on this smoke stack: marker
  # row 1 before the target, row 2 after it. `restore-pitr --promote` ends recovery AT the target
  # (new timeline), `pitr-cutover` installs it as the live cluster. Must hold: only row 1, not in
  # recovery, timeline > 1, WAL archiving healthy (the old §6 procedure crash-recovered past the
  # target on timeline 1 and then failed archiving forever), the timeline history archived, and
  # the whole stack back through `deploy.sh first` (post-checks) with a green watchdog.
  local why42="" tli42 rows42
  lc_psql <<<"CREATE SCHEMA lc_smoke_pitr; CREATE TABLE lc_smoke_pitr.marker (id int PRIMARY KEY); INSERT INTO lc_smoke_pitr.marker VALUES (1);" >/dev/null
  sleep 2
  target=$(lc_psql <<<"SELECT now();")
  sleep 2
  lc_psql <<<"INSERT INTO lc_smoke_pitr.marker VALUES (2);" >/dev/null
  a0=$(lc_psql <<<"SELECT archived_count FROM pg_stat_archiver;")
  lc_psql <<<"SELECT pg_switch_wal();" >/dev/null
  for ((i = 0; i < 30; i++)); do
    a1=$(lc_psql <<<"SELECT archived_count FROM pg_stat_archiver;")
    ((a1 > a0)) && break
    sleep 1
  done
  if ! runc S42 "$LC_SCRIPTS_DIR/pg-ops.sh" restore-pitr --target-time "$target" --promote; then
    why42="restore-pitr --promote failed"
  elif ! grep -q '^pitr_promoted_timeline=[2-9]' "$EV/logs/S42.log"; then
    why42="no promoted timeline reported"
  elif runc S42 "$LC_SCRIPTS_DIR/pg-ops.sh" pitr-cutover; then
    why42="pitr-cutover ran without LC_CONFIRM_REPLACE_LIVE"
  elif ! LC_CONFIRM_REPLACE_LIVE=I_UNDERSTAND_FORWARD_ONLY runc S42 "$LC_SCRIPTS_DIR/pg-ops.sh" pitr-cutover; then
    why42="pitr-cutover failed"
  else
    rows42=$(lc_psql <<<"SELECT count(*) || ':' || max(id) FROM lc_smoke_pitr.marker;" 2>>"$EV/logs/S42.log" || true)
    tli42=$(lc_psql <<<"SELECT timeline_id FROM pg_control_checkpoint();" 2>>"$EV/logs/S42.log" || true)
    if [[ "$rows42" != "1:1" ]]; then
      why42="marker rows=$rows42, expected 1:1 (data after the target came back)"
    elif ! [[ "$tli42" =~ ^[0-9]+$ ]] || ((tli42 < 2)); then
      why42="timeline=$tli42 after cut-over"
    elif [[ ! -f "$LC_BACKUP_DIR/wal/$(printf '%08X.history' "$tli42")" ]]; then
      why42="timeline history not archived"
    elif ! runc S42 "$LC_SCRIPTS_DIR/deploy.sh" --smoke first; then
      why42="deploy.sh first after the cut-over failed"
    elif ! LC_TLS_MIN_DAYS=0 runc S42 "$LC_SCRIPTS_DIR/watchdog.sh"; then
      why42="watchdog not green after the cut-over"
    elif ! grep -q '^W4 PASS archived=[0-9]* failed=0' "$EV/logs/S42.log"; then
      why42="W4 archiver not clean after the cut-over"
    fi
  fi
  if [[ -z "$why42" ]]; then
    rec S42 PASS "promote + cut-over: rows=$rows42 timeline=$tli42, history archived, archiver failed=0, stack back via deploy.sh first, watchdog green"
  else rec S42 FAIL "$why42 (logs/S42.log)"; fi

  # S32 graceful stop
  runc S32 lc_compose stop || true
  bad=$(lc_compose ps -a -q | xargs -r docker inspect -f '{{.Name}} {{.State.ExitCode}}' | awk '$2 == 137 { print $1 }' | tr '\n' ' ')
  if [[ -z "$bad" ]]; then rec S32 PASS "no exit 137 within grace periods"; else rec S32 FAIL "SIGKILLed: $bad"; fi
}

teardown() {
  ((full_started)) || return 0
  if [[ "${LC_SMOKE_KEEP:-0}" == 1 ]]; then
    lc_warn "LC_SMOKE_KEEP=1: stack $PROJECT and $SMOKE_ROOT kept for debugging (remove by hand)"
    return 0
  fi
  # All profiles: a bare --profile would replace COMPOSE_PROFILES and leave services running.
  lc_compose_all down -v --remove-orphans >>"$EV/logs/S33.log" 2>&1 || true
  local left
  left=$(
    docker ps -aq --filter "label=com.docker.compose.project=$PROJECT"
    docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT"
    docker network ls -q --filter "label=com.docker.compose.project=$PROJECT"
  )
  rm -rf "$SMOKE_ROOT"
  if [[ -z "$left" && ! -e "$SMOKE_ROOT" ]]; then rec S33 PASS "no container/volume/network/temp dir left"; else rec S33 FAIL "leftovers remain"; fi
}

finish() {
  local rc=$? status=0
  set +e
  if ((full_started)) && [[ -n "${LC_SECRETS_DIR:-}" && -d "${LC_SECRETS_DIR:-}" ]]; then
    lc_secret_scan "$EV/logs" 2>>"$EV/logs/final-secret-scan.log" || rec SCAN FAIL "secret value found in smoke evidence logs"
  fi
  teardown
  if grep -q $'\tFAIL\t' "$EV/cases.tsv" || ((rc != 0 && rc != 3)); then status=1; elif grep -q $'\tBLOCKED\t' "$EV/cases.tsv"; then status=3; fi
  python3 - "$EV" "$mode" "$started_at" "$(lc_ts)" "$(git -C "$LC_REPO_ROOT" rev-parse HEAD 2>/dev/null)" "$status" <<'PY'
import csv, json, os, platform, subprocess, sys
ev, mode, started, ended, commit, status = sys.argv[1:7]
cases = [dict(zip(("id", "status", "note"), row)) for row in csv.reader(open(os.path.join(ev, "cases.tsv")), delimiter="\t") if row]
cmds = [dict(zip(("case", "exit_code", "command"), row)) for row in csv.reader(open(os.path.join(ev, "commands.tsv")), delimiter="\t") if row]
def ver(cmd):
    try:
        return subprocess.run(cmd, capture_output=True, text=True, timeout=10).stdout.strip().splitlines()[0]
    except Exception:
        return "unavailable"
static_ids = ["S01", "S02", "S03", "S04", "S05", "S06"]
full_ids = static_ids + ["S%02d" % i for i in range(7, 44)] + ["S10a", "S10b", "S10c", "S10d", "S10e", "S29m"]
result = {
    "run_id": os.path.basename(ev), "task_id": "T22", "commit": commit,
    "environment": {"mode": mode, "host": platform.node(), "kernel": platform.release(),
                    "status_vocabulary": "PASS|FAIL|BLOCKED|NOT_RUN; LOCAL smoke, not LIVE/production acceptance"},
    "dependency_pins": {"docker": ver(["docker", "version", "--format", "{{.Server.Version}}"]),
                        "compose": ver(["docker", "compose", "version", "--short"]), "go": ver(["go", "version"]),
                        "images": "see deploy/docker/*.Dockerfile ARG *_IMAGE and deploy/compose.yml image: pins"},
    "actual_commands": [c["command"] for c in cmds],
    "exit_codes": {f'{c["case"]}#{i}': int(c["exit_code"]) for i, c in enumerate(cmds)},
    "fixtures": "temp config/secrets/backup dirs (deleted), *.localhost hosts, generated random secrets only",
    "expected_cases": static_ids if mode == "static" else full_ids,
    "observed_cases": [c["id"] for c in cases],
    "results": cases,
    "artifacts": sorted(os.listdir(os.path.join(ev, "logs"))),
    "started_at": started, "ended_at": ended,
    "reviewer": None,
    "not_run": [c["id"] + ": " + c["note"] for c in cases if c["status"] in ("NOT_RUN", "BLOCKED")],
    "overall": {"0": "PASS", "1": "FAIL", "3": "BLOCKED"}[status],
}
json.dump(result, open(os.path.join(ev, "result.json"), "w"), indent=2, ensure_ascii=False)
PY
  lc_info "evidence: $EV/result.json  overall=$([[ $status == 0 ]] && echo PASS || ([[ $status == 3 ]] && echo BLOCKED || echo FAIL))"
  exit "$status"
}
trap finish EXIT

static_cases
if [[ "$mode" == full ]]; then full_cases; fi
