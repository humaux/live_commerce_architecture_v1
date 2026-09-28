#!/usr/bin/env bash
# File: deploy/scripts/deploy.sh
# Purpose: the three supported release operations (deploy-design §16.1, D11, D12):
#   first               preflight -> images present -> postgres healthy -> migrate ->
#                       provision-logins -> up -d (active profiles) -> post checks -> log
#   upgrade <tag>       preflight(new tag) -> MANDATORY backup -> stop app+workers (Caddy keeps
#                       answering 503 + Retry-After) -> migrate(new tag) -> provision -> up -d
#                       -> post checks -> log. Never auto-rolls back or restores.
#   app-rollback <tag>  allowed ONLY if the migration ledger count equals the one recorded when
#                       <tag> was last deployed; otherwise refuses ("forward-fix only").
# Usage: deploy.sh [--smoke] first | upgrade <tag> | app-rollback <tag>
#   --smoke: used by smoke.sh S37-S39 — public checks use --resolve to 127.0.0.1 and Caddy's
#   local CA instead of real DNS/ACME.
# Runs as/in: deploy host (root), through lib.sh lc_compose. NO production deploy may be run
#   without owner approval (AGENTS.md); this script only automates the approved procedure.
# Reads env: compose.env (IMAGE_TAG, hosts, LC_STATE_DIR, profiles), LC_SMOKE_CACERT (smoke).
# Reads secrets: none directly (postgres/migrate/provision read their own mounted files).
# Writes: ${LC_STATE_DIR:-/var/lib/live-commerce}/deployments.log
#   (UTC ts <TAB> action <TAB> tag <TAB> ledger_count <TAB> operator).
# Used by: operators (docs/runbooks/deploy.md), smoke.sh S37-S39.
# Depends on: preflight.sh, pg-ops.sh (backup), lib.sh; images from build-images.sh.
# Exit: 0 ok; 1 failed (prints the rollback decision tree); 75 migration lock busy (retry
#   later by hand, never in a loop); 2 usage.
# Status: DESIGN; S37-S39 BLOCKED until cmd/migrate exists.
# Change rules: keep the order migrate -> provision -> start (架构.md §22.1 line 731); never add
#   automatic DB restore; edge-netns must never be targeted alone.
set -Eeuo pipefail
# shellcheck source-path=SCRIPTDIR source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

smoke=0
if [[ "${1:-}" == --smoke ]]; then
  smoke=1
  shift
fi
action=${1:-}
arg=${2:-}
lc_load_env "$LC_COMPOSE_ENV"
state_dir=${LC_STATE_DIR:-/var/lib/live-commerce}
deploy_log="$state_dir/deployments.log"
operator=${SUDO_USER:-${USER:-unknown}}
APP_SERVICES=(api admin storefront expiry-worker payment-worker-sandbox payment-worker-live meta-worker)

decision_tree() {
  cat >&2 <<'EOF'
------------------------------------------------------------------------------------------------
Deploy step failed. Decide (docs/runbooks/deploy.md §回滚决策树):
 1. Nothing changed in the DB ledger since the previous tag?  -> deploy.sh app-rollback <previous tag>
 2. Ledger changed and NO external business fact since the pre-upgrade backup (no payments/
    orders after traffic reopened)?  -> owner decides a DB restore (backup-restore.md), then
    app-rollback. Never restore over real payments.
 3. Otherwise -> forward fix + reconciliation (架构.md §22.1). Never edit applied SQL/checksums.
 Diagnostics: deploy/scripts/collect-diagnostics.sh --since 1h
------------------------------------------------------------------------------------------------
EOF
}
trap 'rc=$?; if ((rc != 0 && rc != 2 && rc != 75)); then decision_tree; fi' EXIT

active_app_services() {
  local s
  for s in "${APP_SERVICES[@]}"; do
    if lc_service_active "$s"; then printf '%s\n' "$s"; fi
  done
  return 0
}

run_migrate() {
  local rc=0
  lc_info "migrate (tag $IMAGE_TAG)"
  lc_compose run --rm -T migrate || rc=$?
  if ((rc == 75)); then
    lc_error "migrate exit 75: another migration holds advisory lock 718020260920. Do NOT loop; see docs/runbooks/incident.md §migrate"
    exit 75
  fi
  ((rc == 0)) || lc_die "migrate failed (exit $rc); see: docker compose logs migrate / incident.md §migrate"
}

run_provision() {
  lc_info "provision-logins"
  lc_compose run --rm -T --no-deps provision-logins || lc_die "provision-logins failed (role drift or readiness false; no auto-repair)"
}

wait_postgres() {
  local i status
  lc_compose up -d postgres
  for ((i = 0; i < 60; i++)); do
    status=$(docker inspect -f '{{.State.Health.Status}}' "$(lc_compose ps -q postgres)" 2>/dev/null || echo starting)
    [[ "$status" == healthy ]] && return 0
    sleep 3
  done
  lc_die "postgres not healthy after 180 s"
}

# post_checks — health of long-running services, worker ready tokens, public HTTPS.
post_checks() {
  local since=$1 s cid status i ok token curl_args=()
  for ((i = 0; i < 40; i++)); do
    ok=1
    while IFS= read -r s; do
      cid=$(lc_compose ps -q "$s" 2>/dev/null)
      [[ -n "$cid" ]] || { ok=0 && continue; }
      status=$(docker inspect -f '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$cid")
      [[ "$status" == running/healthy || "$status" == running/none ]] || ok=0
    done < <(lc_long_running_services)
    ((ok)) && break
    sleep 3
  done
  ((ok)) || lc_die "post-check: not all services running/healthy (docker compose ps)"
  for s in expiry-worker:expiry_worker_ready payment-worker-sandbox:payment_worker_ready \
    payment-worker-live:payment_worker_ready meta-worker:meta_worker_ready; do
    token=${s#*:} s=${s%%:*}
    lc_service_active "$s" || continue
    for ((i = 0; i < 30; i++)); do
      grep -q "$token" < <(lc_compose logs --no-log-prefix --since "$since" "$s" 2>&1) && break
      sleep 2
    done
    ((i < 30)) || lc_die "post-check: $s did not log $token within 60 s"
    lc_info "post-check $s ready"
  done
  if lc_service_active caddy; then
    if ((smoke)); then
      # *.localhost sites use Caddy's internal CA; its root appears once Caddy has started.
      LC_SMOKE_CACERT=${LC_SMOKE_CACERT:-$state_dir/smoke-caddy-root.crt}
      for ((i = 0; i < 20; i++)); do
        lc_compose exec -T caddy cat /data/caddy/pki/authorities/local/root.crt >"$LC_SMOKE_CACERT" 2>/dev/null && break
        sleep 2
      done
      [[ -s "$LC_SMOKE_CACERT" ]] || lc_die "post-check: Caddy local root CA not available"
      curl_args=(--resolve "$LC_API_HOST:${LC_HTTPS_PORT}:127.0.0.1" --cacert "$LC_SMOKE_CACERT")
    fi
    for ((i = 0; i < 20; i++)); do
      curl -fsS --max-time 5 "${curl_args[@]}" -o /dev/null "https://$LC_API_HOST:${LC_HTTPS_PORT}/healthz" && break
      sleep 3
    done
    ((i < 20)) || lc_die "post-check: https://$LC_API_HOST/healthz not reachable through the edge"
    lc_info "post-check edge https ok"
  fi
}

log_deploy() {
  local ledger
  ledger=$(lc_ledger_count)
  mkdir -p "$state_dir"
  printf '%s\t%s\t%s\t%s\t%s\n' "$(lc_ts)" "$1" "$IMAGE_TAG" "$ledger" "$operator" >>"$deploy_log"
  lc_info "recorded $1 tag=$IMAGE_TAG ledger_count=$ledger in deployments.log"
}

images_present() {
  local img
  for img in go admin storefront caddy; do
    docker image inspect "${LC_IMAGE_PREFIX:-lc}-$img:$IMAGE_TAG" >/dev/null 2>&1 ||
      lc_die "image ${LC_IMAGE_PREFIX:-lc}-$img:$IMAGE_TAG missing (build-images.sh)"
  done
}

case "$action" in
first)
  since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if ((smoke)); then "$LC_SCRIPTS_DIR/preflight.sh"; else "$LC_SCRIPTS_DIR/preflight.sh" --online; fi
  images_present
  wait_postgres
  run_migrate
  run_provision
  lc_compose up -d
  post_checks "$since"
  log_deploy first
  ;;
upgrade)
  [[ "$arg" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || lc_die "usage: deploy.sh upgrade <tag>" 2
  export IMAGE_TAG=$arg
  since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  "$LC_SCRIPTS_DIR/preflight.sh"
  images_present
  "$LC_SCRIPTS_DIR/pg-ops.sh" backup --tag "pre-upgrade-$IMAGE_TAG" || lc_die "mandatory pre-upgrade backup failed; nothing was changed"
  before=$(lc_ledger_count)
  mapfile -t svcs < <(active_app_services)
  lc_info "maintenance window: stopping ${svcs[*]} (Caddy answers 503 + Retry-After)"
  ((${#svcs[@]} == 0)) || lc_compose stop "${svcs[@]}"
  run_migrate
  run_provision
  lc_compose up -d
  post_checks "$since"
  lc_info "ledger_count before=$before after=$(lc_ledger_count)"
  log_deploy upgrade
  ;;
app-rollback)
  [[ "$arg" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || lc_die "usage: deploy.sh app-rollback <tag>" 2
  [[ -r "$deploy_log" ]] || lc_die "no deployments.log: cannot prove the ledger is unchanged; forward-fix only"
  recorded=$(awk -F'\t' -v t="$arg" '$3 == t { c = $4 } END { print c }' "$deploy_log")
  [[ -n "$recorded" ]] || lc_die "tag $arg was never deployed here; forward-fix only"
  current=$(lc_ledger_count)
  if [[ "$current" != "$recorded" ]]; then
    lc_error "REFUSED: ledger_count now=$current but $arg ran with $recorded. App rollback over a changed schema is unsafe; forward-fix only (runbook §回滚决策树)."
    trap - EXIT
    exit 1
  fi
  export IMAGE_TAG=$arg
  since=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  images_present
  lc_compose up -d
  post_checks "$since"
  log_deploy app-rollback
  ;;
*)
  lc_die "usage: deploy.sh [--smoke] first | upgrade <tag> | app-rollback <tag>" 2
  ;;
esac
trap - EXIT
lc_info "$action done (tag $IMAGE_TAG)"
