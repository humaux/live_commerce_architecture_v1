#!/usr/bin/env bash
# R12 task-owned local gate runner. Never uses live Meta credentials or endpoints.
set -uo pipefail
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution
export LC_TEST_LOCK_WAIT=14400
out=output/ads-attribution/r12-round2
git diff --quiet HEAD -- . ':!output' || exit 90
run() {
  local name="$1"; shift
  local sha rc
  sha="$(git rev-parse HEAD)"
  {
    printf 'source=%s\nstarted=%s\ncommand=' "$sha" "$(date -u +%FT%TZ)"
    printf '%q ' "$@"; printf '\n'
    "$@"
  } > "$out/$name.log" 2>&1
  rc=$?
  printf '%s\t%s\t%s\t%s\n' "$name" "$rc" "$sha" "$(date -u +%FT%TZ)" >> "$out/results.tsv"
  printf '%s exit=%s source=%s\n' "$name" "$rc" "$sha"
  return "$rc"
}
case "${1:-}" in
  static)
    failed=0
    run go-build go build ./... || failed=1
    run go-vet go vet ./... || failed=1
    run gofmt bash -c 'git ls-files -z -- "*.go" | xargs -0 gofmt -l; test -z "$(git ls-files -z -- "*.go" | xargs -0 gofmt -l)"' || failed=1
    run check-gates bash scripts/dev/check-gates.sh || failed=1
    run depmap bash scripts/dev/depmap.sh --check || failed=1
    run test-node bash scripts/dev/test-node.sh || failed=1
    run admin-tsc pnpm --filter admin exec tsc --noEmit || failed=1
    run storefront-tsc pnpm --filter storefront exec tsc --noEmit || failed=1
    run G04 env LC_RELEASE_GATE_OUT="$PWD/$out/g04-final" bash scripts/dev/release-gate.sh --strict --only G04 || failed=1
    exit "$failed";;
  focused)
    run focused bash scripts/dev/test-focused.sh '^(TestAdsAttribution|TestAds|TestMetaAds|TestMetaConnect|TestMetaClaimsMCI10|TestLiveClaimsKC03Schema|TestCustomersBillingCB02|TestCustomersBillingCB04|TestCustomersBillingCB05|TestR2IntegrationUpgradeFromReleaseHead|TestT06WorkerAuthorityAndFunctionACL)' ;;
  browser)
    run browser-attribution bash scripts/dev/test-local.sh --browser-ads-attribution ;;
  sweep)
    run click-sweep bash scripts/dev/test-local.sh --browser-click-sweep ;;
  g07)
    (while true; do date -u +%FT%TZ; uptime; sleep 30; done) >> "$out/g07-machine-load.log" &
    sampler=$!
    trap 'kill "$sampler" 2>/dev/null || true' EXIT
    run G07-console env LC_RELEASE_GATE_OUT="$PWD/$out/g07-final" bash scripts/dev/release-gate.sh --strict --only G07 ;;
  *) printf 'usage: %s static|focused|browser|sweep|g07\n' "$0"; exit 2;;
esac
