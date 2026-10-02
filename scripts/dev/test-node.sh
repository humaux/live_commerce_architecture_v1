#!/usr/bin/env bash
# test-node.sh — the gate for every Node unit suite that needs no Docker/PG/browser (unit maintainability).
# Runs (always): apps/storefront/tests/*.test.mjs, packages/i18n/tests/*.test.ts, packages/markdown-lite/tests/*.test.ts, tests/admin/design-model.test.ts (store-design) and tests/admin/team-model.test.ts (staff-team) and tests/admin/design-gate.test.ts (store-design gate). tests/admin/promotions-model.test.ts tests/admin/team-bff.test.ts tests/admin/notify-model.test.ts tests/admin/merchant-tools-model.test.ts tests/admin/meta-connect-gate.test.ts (MCG09 callback/open-redirect pure functions) tests/deploy/meta-connect-preflight.test.mjs (MCG08 preflight negatives for the COMMERCE_META_LOGIN_* keys) tests/admin/invite-next.test.ts (staff-invite return-to validator) tests/admin/money-time-model.test.ts (stop-bleed D02 TWD whole-dollar money rule + M06 Taipei time)
# Runs (only with a pinned binary): tests/media/r04-input-runner.test.mjs needs COMMERCE_R04_LIVEKIT_BINARY;
#   without it that suite is reported NOT_RUN (never PASS). CI has no binary, so CI reports NOT_RUN for r04;
#   pass --require-r04 to make a missing binary exit 2 instead (release/acceptance runs).
# Usage: bash scripts/dev/test-node.sh [--require-r04]
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
if [[ "$#" -gt 1 ]] || [[ "$#" -eq 1 && "$1" != --require-r04 ]]; then
  echo "Usage: bash scripts/dev/test-node.sh [--require-r04]" >&2; exit 2
fi
node --test --experimental-strip-types apps/storefront/tests/*.test.mjs packages/i18n/tests/*.test.ts packages/markdown-lite/tests/*.test.ts tests/admin/design-model.test.ts tests/admin/team-model.test.ts tests/admin/design-gate.test.ts tests/admin/promotions-model.test.ts tests/admin/team-bff.test.ts tests/admin/notify-model.test.ts tests/admin/meta-connect-model.test.ts tests/admin/merchant-tools-model.test.ts tests/admin/meta-connect-gate.test.ts tests/deploy/meta-connect-preflight.test.mjs tests/admin/invite-next.test.ts tests/admin/money-time-model.test.ts tests/admin/shell-registry.test.ts tests/admin/shell-architecture.test.mjs
if [[ -n "${COMMERCE_R04_LIVEKIT_BINARY:-}" ]]; then
  node --test tests/media/r04-input-runner.test.mjs
else
  echo "NOT_RUN: tests/media/r04-input-runner.test.mjs (COMMERCE_R04_LIVEKIT_BINARY unset)" >&2
  [[ "${1:-}" != --require-r04 ]] || exit 2
fi
