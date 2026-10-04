#!/usr/bin/env bash
# Task-owned evidence runner. No production endpoints, credentials or mutations.
set -uo pipefail
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution
export LC_TEST_LOCK_WAIT=14400
out=output/ads-attribution/r13-review
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
git diff --quiet HEAD -- . ':!output' || exit 90
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
    run focused bash scripts/dev/test-focused.sh '^(TestAdsAttribution|TestAds|TestMetaAds|TestMetaConnect|TestMetaClaimsMCI10|TestLiveClaimsKC03Schema|TestR2IntegrationUpgradeFromReleaseHead|TestT06WorkerAuthorityAndFunctionACL)' ;;
  browser)
    run browser-attribution bash scripts/dev/test-local.sh --browser-ads-attribution ;;
  sweep)
    run click-sweep bash scripts/dev/test-local.sh --browser-click-sweep ;;
  edge)
    run platform-edge env LC_PLATFORM_EDGE_EVIDENCE="$PWD/$out/platform-edge" node tests/deploy/platform-edge.mjs ;;
  g07)
    # This PID belongs to this runner only. Never kill a test or foreign process.
    (while true; do date -u +%FT%TZ; uptime; sleep 30; done) >> "$out/g07-machine-load.log" &
    sampler=$!
    trap 'kill "$sampler" 2>/dev/null || true' EXIT
    export LC_RELEASE_GATE_OUT="$PWD/$out/g07-final"
    run G07-console bash scripts/dev/release-gate.sh --strict --only G07 ;;
  *) printf 'usage: %s static|focused|browser|sweep|edge|g07\n' "$0"; exit 2;;
esac
