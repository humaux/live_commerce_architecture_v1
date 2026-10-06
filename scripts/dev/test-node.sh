#!/usr/bin/env bash
# Purpose: run all registered DB-free Node acceptance suites.
# Depends on: Node24 and frozen workspace packages; optional pinned media binary.
# Used by: developers and CI; tracking-backfill pure properties included.
# test-node.sh — the gate for every Node unit suite that needs no Docker/PG/browser (unit maintainability).
# Runs (always): apps/storefront/tests/*.test.mjs, packages/i18n/tests/*.test.ts, packages/markdown-lite/tests/*.test.ts, tests/admin/design-model.test.ts (store-design) and tests/admin/team-model.test.ts (staff-team) and tests/admin/design-gate.test.ts (store-design gate). tests/admin/promotions-model.test.ts tests/admin/team-bff.test.ts tests/admin/notify-model.test.ts tests/admin/merchant-tools-model.test.ts tests/admin/tracking-backfill-model.test.ts tests/admin/meta-connect-gate.test.ts (MCG09 callback/open-redirect pure functions) tests/deploy/meta-connect-preflight.test.mjs (MCG08 preflight negatives for the COMMERCE_META_LOGIN_* keys) tests/deploy/ops-alert-preflight.test.mjs (ops-disk-guard D4: preflight P08 alert-channel rule) tests/admin/invite-next.test.ts (staff-invite return-to validator) tests/admin/money-time-model.test.ts (stop-bleed D02 TWD whole-dollar money rule + M06 Taipei time)
# Runs (only with a pinned binary): tests/media/r04-input-runner.test.mjs needs COMMERCE_R04_LIVEKIT_BINARY;
#   without it that suite is reported NOT_RUN (never PASS). CI has no binary, so CI reports NOT_RUN for r04;
#   pass --require-r04 to make a missing binary exit 2 instead (release/acceptance runs).
# Usage: bash scripts/dev/test-node.sh [--require-r04]
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
if [[ "$#" -gt 1 ]] || [[ "$#" -eq 1 && "$1" != --require-r04 ]]; then
  echo "Usage: bash scripts/dev/test-node.sh [--require-r04]" >&2; exit 2
fi
node --test --experimental-strip-types apps/storefront/tests/*.test.mjs packages/i18n/tests/*.test.ts packages/markdown-lite/tests/*.test.ts tests/admin/design-model.test.ts tests/admin/team-model.test.ts tests/admin/design-gate.test.ts tests/admin/promotions-model.test.ts tests/admin/team-bff.test.ts tests/admin/notify-model.test.ts tests/admin/meta-connect-model.test.ts tests/admin/merchant-tools-model.test.ts tests/admin/meta-connect-gate.test.ts tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs tests/admin/invite-next.test.ts tests/admin/money-time-model.test.ts tests/admin/store-domains-ui.test.ts tests/admin/shell-registry.test.ts tests/admin/shell-architecture.test.mjs tests/ui/click-sweep-lib.test.mjs tests/ui/visual-lint-lib.test.mjs
node --test --experimental-strip-types tests/admin/orders-v2.test.ts
node --test --experimental-strip-types tests/admin/store-number.test.ts
node --test --experimental-strip-types tests/admin/platform-site.test.ts
node --test --experimental-strip-types tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts tests/admin/attribution-format.test.ts
node --test --experimental-strip-types tests/admin/product-document.test.ts tests/admin/product-patch.test.ts tests/admin/product-media-model.test.ts tests/admin/backend-parity.test.ts
node --test --experimental-transform-types tests/admin/catalog-receipt.test.ts
if [[ -n "${COMMERCE_R04_LIVEKIT_BINARY:-}" ]]; then
  node --test tests/media/r04-input-runner.test.mjs
else
  echo "NOT_RUN: tests/media/r04-input-runner.test.mjs (COMMERCE_R04_LIVEKIT_BINARY unset)" >&2
  [[ "${1:-}" != --require-r04 ]] || exit 2
fi

# W3-U1b pure selection, wire and request boundary negatives (no browser).
node --test --experimental-strip-types tests/admin/picklist-model.test.ts
