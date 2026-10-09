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
node --test --experimental-strip-types apps/storefront/tests/*.test.mjs packages/i18n/tests/*.test.ts packages/markdown-lite/tests/*.test.ts tests/admin/design-model.test.ts tests/admin/team-model.test.ts tests/admin/design-gate.test.ts tests/admin/promotions-model.test.ts tests/admin/team-bff.test.ts tests/admin/notify-model.test.ts tests/admin/meta-health-model.test.ts tests/admin/meta-connect-model.test.ts tests/admin/merchant-tools-model.test.ts tests/admin/meta-connect-gate.test.ts tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs tests/deploy/deploy-prep-r3.test.mjs tests/admin/invite-next.test.ts tests/admin/money-time-model.test.ts tests/admin/store-domains-ui.test.ts tests/admin/shell-registry.test.ts tests/admin/shell-render.test.ts tests/admin/shell-architecture.test.mjs tests/ui/click-sweep-lib.test.mjs tests/ui/visual-lint-lib.test.mjs
node --test --experimental-strip-types tests/admin/operations-model.test.ts
# TZ audit: real schedule/default producers and loaded component display seams run in both foreign zones.
for tz_audit_zone in UTC America/Los_Angeles; do
  TZ="$tz_audit_zone" node --test --experimental-strip-types tests/admin/ads-model.test.ts tests/admin/customers-model.test.ts tests/admin/timezone-ui.test.mjs apps/storefront/tests/timezone.test.mjs
done
node --test --experimental-strip-types tests/admin/orders-v2.test.ts
node --test --experimental-strip-types tests/admin/store-number.test.ts
node --test --experimental-strip-types tests/admin/platform-site.test.ts
node --test --experimental-strip-types tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts tests/admin/attribution-format.test.ts
# W4-U1 platform card payments + settlements: parsers (0137/0150 signs), BFF grammar, copy parity, display state machine (apps/storefront/tests/payment-collector.test.mjs runs in the first glob)
node --test --experimental-strip-types tests/admin/card-payments-model.test.ts tests/admin/card-payments-request.test.ts tests/admin/card-payments-copy.test.ts tests/admin/card-payments-wire.test.ts
node --test --experimental-strip-types tests/admin/product-document.test.ts tests/admin/product-patch.test.ts tests/admin/product-media-model.test.ts tests/admin/backend-parity.test.ts
node --test --experimental-transform-types tests/admin/catalog-receipt.test.ts
node --test --experimental-transform-types tests/admin/live-workspace.test.ts tests/admin/live-console-model.test.ts tests/admin/live-console-bff.test.ts
node --test --experimental-strip-types tests/admin/comment-stream.test.ts tests/admin/comment-stream-hooks.test.ts
node --test --experimental-strip-types tests/admin/live-workspace-hooks.test.ts tests/admin/live-console-render.test.ts
node --test --experimental-strip-types tests/admin/claims-backend-parity.test.ts tests/admin/claims-model.test.ts tests/admin/claims-request.test.ts
if [[ -n "${COMMERCE_R04_LIVEKIT_BINARY:-}" ]]; then
  node --test tests/media/r04-input-runner.test.mjs
else
  echo "NOT_RUN: tests/media/r04-input-runner.test.mjs (COMMERCE_R04_LIVEKIT_BINARY unset)" >&2
  [[ "${1:-}" != --require-r04 ]] || exit 2
fi

# PM-U pure resize/query/request contract (no browser).
node --test --experimental-strip-types tests/admin/product-media-ui-model.test.ts
node --test --experimental-transform-types tests/admin/photo-preprocess.test.ts
# W3-U1b pure selection, wire and request boundary negatives (no browser).
node --test --experimental-strip-types tests/admin/picklist-model.test.ts
# W3-U4 parcel merge: DTO parsers + reconcile + copy ruling pins, BFF route grammar, and the REAL route handler against a stub upstream (no browser/PG).
node --test --experimental-strip-types tests/admin/parcels-model.test.ts tests/admin/parcels-request.test.ts tests/admin/parcels-bff.test.ts tests/admin/parcels-scope.test.ts tests/admin/parcels-discovery.test.ts

# W6-U1 frozen tags/notes/report DTOs, exact authority fences, real component effects and integration (no browser/PG).
node --test --experimental-strip-types tests/admin/customer-tags-bff.test.ts tests/admin/customer-tags-proxy.test.ts \
  tests/admin/customer-tags-ui.test.mjs tests/admin/customer-tags-client.test.mjs tests/admin/customer-tags-draft.test.mjs tests/admin/guarded-read-logout.test.mjs tests/admin/customer-tags-write.test.mjs tests/admin/customer-tags-scope.test.mjs \
  tests/admin/reports-bff.test.ts tests/admin/reports-render.test.mjs tests/admin/w6-integration.test.ts tests/admin/w6-route-seam.test.mjs

# W5-U1 immutable raw CSV import, private row projection, read-only archive and form lifecycle (no browser/PG).
node --test --experimental-strip-types tests/admin/import-wire-model.test.ts tests/admin/import-wire-header.test.ts \
  tests/admin/import-client.test.mjs tests/admin/import-history-model.test.ts tests/admin/import-history-client.test.mjs \
  tests/admin/import-history-bff.test.mjs tests/admin/import-history-ui.test.mjs tests/admin/import-view.test.ts \
  tests/admin/import-integration.test.ts tests/admin/import-coordinator.test.mjs tests/admin/import-readiness.test.mjs

# CI speed-up (2026-10-07): LC_SWEEP_SHARD partition + whole-run aggregate (click sweep, visual lint), the foundation shard plan and the gates.yml matrix planner. No browser, no PG.
node --test --experimental-strip-types tests/ui/sweep-shard-lib.test.mjs
node --test tests/ci/shard-plan.test.mjs tests/ci/ci-plan.test.mjs tests/ci/pr-modes.test.mjs tests/ci/mode-registry.test.mjs

# LC-U2b browser registration (DB-free); the real UI spec is CI-only and must select its own authority suite.
node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts tests/admin/inbox-copy.test.ts tests/admin/inbox-bff.test.ts
# tests/admin/inbox-review-host.test.ts is the imported hook/child host for both actual-source review suites below.
# tests/admin/inbox-recovery-env.test.ts observes actual client completion and supplies optional CI timing/date profiles.
node --test --experimental-strip-types tests/admin/inbox-review.test.ts tests/admin/inbox-review-recovery.test.ts tests/admin/inbox-window.test.ts tests/admin/inbox-evidence.test.ts tests/admin/inbox-entry.test.ts tests/admin/inbox-browser-source.test.ts tests/admin/inbox-unknown.test.ts tests/admin/inbox-bundle-browser.test.ts tests/admin/inbox-discovery.test.ts tests/admin/inbox-credential-artifacts.test.ts tests/admin/inbox-shell-navigation.test.ts tests/admin/inbox-recovery-locale.test.ts tests/admin/inbox-browser-session.test.ts
node --input-type=module -e 'import assert from "node:assert/strict"; import {readFileSync,existsSync} from "node:fs"; assert.ok(existsSync("tests/foundation/browser_inbox_ui_test.go")); assert.ok(existsSync("tests/admin/inbox-ui.spec.ts")); assert.ok(readFileSync("playwright.config.ts","utf8").includes(`"inbox": ["inbox-ui.spec.ts", "inbox-bundle-ui.spec.ts"]`),"inbox authority suite missing"); assert.ok(readFileSync("scripts/dev/test-local.sh","utf8").includes("TestBrowserInboxUIRealChain"),"inbox Go gate missing"); assert.ok(readFileSync("docs/delivery/GATES.md","utf8").includes("--browser-inbox"),"inbox gate registry missing");'

# LC-U3 exact A15/A16 seams, immutable retries and localized money-path guidance.
node --test --experimental-strip-types tests/admin/create-order-model.test.ts tests/admin/create-order-client.test.ts tests/admin/create-order-bff.test.ts tests/admin/create-order-copy.test.ts
node --test --experimental-transform-types tests/admin/create-order-attempt.test.ts
