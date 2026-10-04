#!/usr/bin/env bash
# Root independent checks for directly touched adapter and customer boundaries.
set -uo pipefail
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution
export LC_TEST_LOCK_WAIT=14400
out=output/ads-attribution/r13-review
git diff --quiet HEAD -- . ':!output' || exit 90
sha="$(git rev-parse HEAD)"
run() {
  local name="$1"; shift
  {
    printf 'source=%s\nstarted=%s\ncommand=' "$sha" "$(date -u +%FT%TZ)"
    printf '%q ' "$@"; printf '\n'
    "$@"
  } > "$out/$name.log" 2>&1
  local rc=$?
  printf '%s\t%s\t%s\t%s\n' "$name" "$rc" "$sha" "$(date -u +%FT%TZ)" >> "$out/boundary-results.tsv"
  printf '%s exit=%s source=%s\n' "$name" "$rc" "$sha"
  return "$rc"
}
run adapter-race go test -race ./internal/attribution/capiroute ./internal/integrations/meta_ads ./internal/metaconnect || exit $?
run focused-customer-boundaries bash scripts/dev/test-focused.sh '^(TestCustomersBillingCB02|TestCustomersBillingCB04|TestCustomersBillingCB05)' || exit $?
