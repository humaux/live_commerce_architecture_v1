#!/usr/bin/env bash
# Purpose: install the mode's browser dependencies and binaries in CI.
# Depends on: frozen Playwright CLI and hosted Linux runner tooling.
# Used by: gates.yml and fake-network-edge process tests.
set -euo pipefail
[[ "$(uname -s)" == Linux ]] || { echo 'ci-playwright-install: Linux runner required' >&2; exit 2; }
[[ "$*" == chromium || "$*" == 'chromium webkit' ]] || { echo 'ci-playwright-install: invalid browser set' >&2; exit 2; }
# Directly declared package; pnpm does not expose the transitive playwright package at the repository root.
node_bin="$(command -v node)"
cli="$(node -p 'require.resolve("@playwright/test/cli")')"
started=$SECONDS
rc=1
for attempt in 1 2 3; do
  echo "ci-playwright-install: dependencies attempt=$attempt/3 browsers=$* limit=240s kill-after=15s"
  # Run both the timeout monitor and CLI as root: Playwright then avoids nested sudo,
  # and the monitor can terminate its complete apt process group. Never use --foreground.
  if sudo timeout --kill-after=15s 240s "$node_bin" "$cli" install-deps "$@"; then rc=0; break; else rc=$?; fi
  echo "ci-playwright-install: dependencies attempt=$attempt exit=$rc elapsed=$((SECONDS-started))s" >&2
  [[ "$attempt" == 3 ]] || sleep 10
done
[[ "$rc" == 0 ]] || { echo "ci-playwright-install: dependencies failed after 3 attempts (exit=$rc)" >&2; exit "$rc"; }
# Browser downloads use the user's versioned cache; system dependencies are never inferred from a cache hit.
pnpm exec playwright install "$@"
echo "ci-playwright-install: browsers=$* elapsed=$((SECONDS-started))s"
