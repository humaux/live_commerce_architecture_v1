#!/usr/bin/env bash
# test-node.sh — the gate for every Node unit suite that needs no Docker/PG/browser (unit maintainability).
# Runs (always): apps/storefront/tests/*.test.mjs, packages/i18n/tests/*.test.ts, packages/markdown-lite/tests/*.test.ts, tests/admin/design-model.test.ts (store-design) and tests/admin/team-model.test.ts (staff-team) and tests/admin/design-gate.test.ts (store-design gate).
# Runs (only with a pinned binary): tests/media/r04-input-runner.test.mjs needs COMMERCE_R04_LIVEKIT_BINARY;
#   without it that suite is reported NOT_RUN (never PASS). CI has no binary, so CI reports NOT_RUN for r04;
#   pass --require-r04 to make a missing binary exit 2 instead (release/acceptance runs).
# Usage: bash scripts/dev/test-node.sh [--require-r04]
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
if [[ "$#" -gt 1 ]] || [[ "$#" -eq 1 && "$1" != --require-r04 ]]; then
  echo "Usage: bash scripts/dev/test-node.sh [--require-r04]" >&2; exit 2
fi
node --test --experimental-strip-types apps/storefront/tests/*.test.mjs packages/i18n/tests/*.test.ts packages/markdown-lite/tests/*.test.ts tests/admin/design-model.test.ts tests/admin/team-model.test.ts tests/admin/design-gate.test.ts
if [[ -n "${COMMERCE_R04_LIVEKIT_BINARY:-}" ]]; then
  node --test tests/media/r04-input-runner.test.mjs
else
  echo "NOT_RUN: tests/media/r04-input-runner.test.mjs (COMMERCE_R04_LIVEKIT_BINARY unset)" >&2
  [[ "${1:-}" != --require-r04 ]] || exit 2
fi
