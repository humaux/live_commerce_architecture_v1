#!/usr/bin/env bash
# Local P2-N1 copy gates; all hosts/contact values are synthetic.
set -uo pipefail
cd /Volumes/data/live_commerce_architecture_v1/.worktrees/platform-messaging-copy
out=output/platform-messaging-copy
export LC_TEST_LOCK_WAIT=14400
export LC_PLATFORM_HOST=platform.example.invalid
export LC_ADMIN_HOST=admin.example.invalid
export LC_COMPANY_CONTACT_EMAIL=contact@example.invalid
export LC_PLATFORM_EVIDENCE="$PWD/$out/browser"
run() {
  local name="$1"; shift
  local sha rc
  sha="$(git rev-parse HEAD)"
  { printf 'source=%s\nstarted=%s\ncommand=' "$sha" "$(date -u +%FT%TZ)"; printf '%q ' "$@"; printf '\n'; "$@"; } > "$out/$name.log" 2>&1
  rc=$?
  printf '%s\t%s\t%s\t%s\n' "$name" "$rc" "$sha" "$(date -u +%FT%TZ)" >> "$out/results.tsv"
  printf '%s exit=%s source=%s\n' "$name" "$rc" "$sha"
  return "$rc"
}
case "${1:-}" in
  red) run red-copy node --test --test-reporter=spec --experimental-strip-types tests/admin/platform-site.test.ts ;;
  static)
    run node-copy node --test --test-reporter=spec --experimental-strip-types tests/admin/platform-site.test.ts || exit $?
    run admin-tsc pnpm --filter admin exec tsc --noEmit || exit $?
    run check-gates bash scripts/dev/check-gates.sh || exit $?
    run copy-detector node /Users/luolimo/.codex/skills/impeccable/scripts/detect.mjs --json apps/admin/lib/platform-legal.ts || exit $?
    ;;
  browser) run browser-platform-site bash scripts/dev/test-local.sh --browser-platform-site ;;
  *) exit 2 ;;
esac
