#!/usr/bin/env bash
# Purpose: one declarative case registry for mode discovery, preparation, builds and execution.
# Depends on: Bash3.2+, test-local-runtime.sh, Docker/Go/Node and each mode's declared tools.
# Used by: developers, CI, pr-modes.mjs, check-gates.sh and release-gate.sh.
# Add a mode in ONE case entry below; browser arms declare lc_browsers for CI dependency selection.
set -euo pipefail
# Keep the runtime guard before cd, exactly as in the original runner; metadata paths are inert.
if [[ "${1:-}" != --list && "${1:-}" != --dry-run ]]; then
  if env | grep -qE '=(sk|rk)_live_'; then echo 'refused: live key in test environment' >&2; exit 2; fi
fi
cd "$(dirname "$0")/../.."
# BEGIN MODE REGISTRY
lc_select_mode() {
case "$1" in
  --browser-inbox)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/claims internal/inbox internal/mail internal/metaconnect"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_inbox_ui_test.go
  test -f tests/admin/inbox-ui.spec.ts
  mkdir -p output/playwright
    }
    lc_run() {
  # The privacy gate uses trusted native tab visibility, requiring headed Chromium.
  inbox_browser_command=(go)
  if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
    command -v xvfb-run >/dev/null || { printf 'INU05 needs xvfb-run on Linux without DISPLAY\n' >&2; exit 2; }
    inbox_browser_command=(xvfb-run --auto-servernum go)
  fi
  LC_BROWSER_INBOX_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 "${inbox_browser_command[@]}" test -race -tags browser -count=1 -timeout=540s -run '^TestBrowserInboxUIRealChain$' -v ./tests/foundation
  printf 'PASS: LC-U2b BROWSER MOCK signed OIDC + Next + Go + PG inbox; not LIVE Meta acceptance.\n'
    }
    ;;
  --browser-meta-health-ui)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/claims internal/integrations/meta/oauth internal/live internal/metaconnect"
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  # brfEvidence owns meta-health-ui/<run>; its durable copy must have a distinct parent.
  LC_META_HEALTH_EVIDENCE_ROOT="$LC_BROWSER_EVIDENCE_ROOT/meta-health-ui-durable" LC_BROWSER_META_HEALTH_UI=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=900s -run '^(TestBrowserMetaHealthUI|TestMetaHealthUIWireRequiresCapability)$' -v ./tests/foundation
    }
    ;;
  --browser-tracking-backfill)
    lc_build=admin
    lc_fixture=none
    lc_covers="internal/claims internal/merchanttools"
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  # Reuse the focused runner's machine-wide queue before creating any PG fixture.
  LC_TRACKING_EVIDENCE_ROOT="$LC_BROWSER_EVIDENCE_ROOT/tracking-backfill" LC_BROWSER_TRACKING_BACKFILL=1 LC_FOCUSED_TAGS=browser LC_FOCUSED_TIMEOUT=1000s bash scripts/dev/test-focused.sh '^TestBrowserTrackingBackfill$'
  printf 'PASS: tracking backfill real clicks, signed MOCK IdP/Stripe and real PG; no LIVE shipments or mail acceptance.\n'
  exit 0
    }
    ;;
  --browser-platform-site)
    lc_build=none
    lc_fixture=none
    lc_covers=""
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  # Share the browser/PG gate queue: rebuilding this checkout while another
  # browser run is using its standalone output invalidates that run's evidence.
  source scripts/dev/test-lock.sh
  if ! lc_lock_acquire "${LC_TEST_LOCK_WAIT:-300}"; then
    printf 'platform-site: test lock busy; no build started\n' >&2
    exit 2
  fi
  trap lc_lock_release EXIT
  mkdir -p "$LC_PLATFORM_EVIDENCE"
  node --test --test-reporter=spec --experimental-strip-types tests/admin/platform-site.test.ts
  pnpm --filter admin build > "$LC_PLATFORM_EVIDENCE/build.log" 2>&1
  node tests/admin/platform-runner.mjs
  exit 0
    }
    ;;
  --meta-health)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  # w1-01b-meta-health (contracts/meta-connection-health-v1.md §11.2): the probe sweep + banner routes against REAL_PG and the
  # dedicated fake Graph (MCH02-MCH09), plus the pure Derive/reader units (MCH01). A selector matching no test is not a gate.
  test -f tests/foundation/meta_health_test.go
  test -f internal/metaconnect/derive_test.go
  grep -q '^func TestMetaHealth' tests/foundation/meta_health_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=240s -run '^TestMetaHealth' -v ./tests/foundation
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=60s ./internal/metaconnect
  printf 'PASS: meta-connection-health backend (MCH01-MCH09): REAL_PG probe sweep (v2 HPKE + v1 AES custody) + B1/B2 banner routes against the fake Graph, and the pure Derive/reader units; Meta = MOCK loopback fake (evidence MOCK), no real Meta traffic; MCH10-MCH12 are NOT_RUN (see docs/delivery/GATES.md).\n'
    }
    ;;
  --browser-admin-shell)
    lc_build=none
    lc_fixture=none
    lc_covers=""
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  mkdir -p "$LC_BROWSER_EVIDENCE_ROOT/ui-w0-shell"
  # --test-reporter=spec pins the "ℹ pass N / ℹ fail F" summary that scripts/dev/release-gate.sh counts (B-browser-admin-shell has no go test events).
  node --test --test-reporter=spec --experimental-strip-types tests/admin/shell-registry.test.ts tests/admin/shell-architecture.test.mjs
  pnpm --filter @live-commerce/admin build > "$LC_BROWSER_EVIDENCE_ROOT/ui-w0-shell/build.log" 2>&1
  node tests/admin/shell-runner.mjs
  exit 0
    }
    ;;
  --browser-click-sweep)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/billing/billingtest internal/buyerhttp internal/claims internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/meta/oauth internal/integrations/meta_ads internal/integrations/metabridge internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/live internal/mail internal/metaconnect internal/msgtemplates"
    lc_browsers=chromium
    lc_prepare() {
  # G-UI8 (BROWSER, MOCK): the real-click sweep over every admin registry route and every storefront route. Refuse a no-test success before any build.
  test -f tests/foundation/browser_click_sweep_test.go
  grep -q '^func TestBrowserClickSweep' tests/foundation/browser_click_sweep_test.go
  test -f tests/ui/click-sweep.mjs
  test -f tests/ui/click-sweep-lib.mjs
  test -f tests/ui/click-sweep-known-defects.json
  node --test tests/ui/click-sweep-lib.test.mjs
  # LC_SWEEP_SHARD=i/N (CI): reject a malformed value before the stack is built (the runner would only throw after the Next builds).
  node --input-type=module -e 'import { parseShard } from "./tests/ui/sweep-shard-lib.mjs"; parseShard(process.argv[1])' "${LC_SWEEP_SHARD:-}"
  mkdir -p "$LC_SWEEP_OUT"
    }
    lc_run() {
  # One Go test owns PG + the Go API + both Next builds' processes and the seed; the runner (tests/ui/click-sweep.mjs) drives Chromium by real clicks.
  LC_BROWSER_CLICK_SWEEP_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=5400s -run '^TestBrowserClickSweep' -v ./tests/foundation
  # PS5: public routes are separate from the admin registry; include every public
  # route in all three locales and both widths, with their own real-click ledger.
  # Sharded (LC_SWEEP_SHARD=i/N) it is independent of the sweep slice, so exactly shard 1 runs it; the marker is what sweep-aggregate.mjs checks.
  if [[ -z "${LC_SWEEP_SHARD:-}" || "${LC_SWEEP_SHARD%%/*}" == 1 ]]; then
    node tests/admin/platform-runner.mjs
    printf '%s\n' "$(git rev-parse HEAD)" > "$LC_SWEEP_OUT/platform-runner.pass"
  fi
  if [[ -n "${LC_SWEEP_SHARD:-}" ]]; then
    printf 'PASS: G-UI8 real-click sweep SHARD %s (this slice only: the whole-run verdict is `node tests/ui/sweep-aggregate.mjs click` over every shard); ledger in %s; signed MOCK IdP, MOCK payments/carrier/Meta, no provider or deployment acceptance.\n' "$LC_SWEEP_SHARD" "$LC_SWEEP_OUT"
  else
  printf 'PASS: G-UI8 real-click sweep (every admin registry route and storefront route at 1586x992 + 390x844 zh-TW and en desktop, 5 click journeys); ledger in %s; signed MOCK IdP, MOCK payments/carrier/Meta, no provider or deployment acceptance.\n' "$LC_SWEEP_OUT"
  fi
    }
    ;;
  --browser-visual-lint)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/billing/billingtest internal/buyerhttp internal/claims internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/meta/oauth internal/integrations/meta_ads internal/integrations/metabridge internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/live internal/mail internal/metaconnect internal/msgtemplates"
    lc_browsers=chromium
    lc_prepare() {
  # G-UI9 (BROWSER, MOCK): screenshot corpus + deterministic layout lint on the click-sweep stack. Refuse a no-test success before any build, and prove the
  # lint can observe a failure (canary: a known-bad page trips every rule, a known-good page trips none) before it is allowed to say a page is clean.
  test -f tests/foundation/browser_click_sweep_test.go
  grep -q '^func TestBrowserClickSweep' tests/foundation/browser_click_sweep_test.go
  test -f tests/ui/visual-audit.mjs
  test -f tests/ui/visual-lint-lib.mjs
  node --test tests/ui/visual-lint-lib.test.mjs
  node tests/ui/visual-lint-canary.mjs
  node --input-type=module -e 'import { parseShard } from "./tests/ui/sweep-shard-lib.mjs"; parseShard(process.argv[1])' "${LC_SWEEP_SHARD:-}"
  mkdir -p "$LC_SWEEP_OUT" "$LC_BROWSER_EVIDENCE_ROOT/ui-visual-audit"
    }
    lc_run() {
  # The click-sweep Go test seeds the stack and starts tests/ui/click-sweep.mjs, which hands over to tests/ui/visual-audit.mjs when LC_SWEEP_ONLY=visual-audit.
  # The audit has no click journeys. Supply its empty fixture only in this run's
  # ignored sweep directory; historical journeys and global LATEST stay untouched.
  printf '{}\n' > "$LC_SWEEP_OUT/journeys.json"
  rm -f "$LC_BROWSER_EVIDENCE_ROOT/ui-visual-audit/LATEST"
  va_rc=0
  LC_SWEEP_ONLY=visual-audit LC_BROWSER_CLICK_SWEEP_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=5400s -run '^TestBrowserClickSweep' -v ./tests/foundation || va_rc=$?
  va_dir="$(cat "$LC_BROWSER_EVIDENCE_ROOT/ui-visual-audit/LATEST" 2>/dev/null || true)"
  if [[ -z "$va_dir" || ! -f "$va_dir/lint.json" ]]; then
    printf 'FAIL: G-UI9 visual lint wrote no lint.json (the stack or the runner stopped early, go test exit %s; see %s/click-sweep/*/click-sweep.mjs.log)\n' "$va_rc" "$LC_BROWSER_EVIDENCE_ROOT" >&2
    exit 1
  fi
  va_verdict="$(node -e 'const r=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));console.log(`shots ${r.shots.captured}/${r.shots.expected} (NOT_RUN ${r.shots.notRun.length}); instances ${Object.entries(r.totals).map(([k,v])=>k+" "+v.instances).join(", ")}; blocking ${r.blockingInstances}; ${r.verdict.reasons.join("; ")||"no blocking finding"}`);process.exit(r.verdict.exit)' "$va_dir/lint.json")" && va_lint=0 || va_lint=$?
  printf 'G-UI9: %s\nG-UI9 evidence: %s (lint.md, lint.json, index.json, shots/, crops/)\n' "$va_verdict" "$va_dir"
  if [[ "$va_lint" != 0 || "$va_rc" != 0 ]]; then
    printf 'FAIL: G-UI9 visual lint (lint verdict exit %s, go test exit %s): blocking violations (R1 R2 R3 R6 R9, R7 clipped controls), a missing shot or a page that did not load; R4 R5 R8 R10 R11 and clipped labels are WARN.\n' "$va_lint" "$va_rc" >&2
    exit 1
  fi
  if [[ -n "${LC_SWEEP_SHARD:-}" ]]; then
    printf 'PASS: G-UI9 visual lint SHARD %s (this slice only: the whole-matrix verdict is `node tests/ui/sweep-aggregate.mjs visual` over every shard); signed MOCK IdP, MOCK payments/carrier/Meta, no provider or deployment acceptance.\n' "$LC_SWEEP_SHARD"
  else
  printf 'PASS: G-UI9 visual lint (every admin registry route, buyer storefront route and platform-site page at 1586x992 + 390x844 in zh-TW, zh-CN and en, shot and measured against R1-R11); signed MOCK IdP, MOCK payments/carrier/Meta, no provider or deployment acceptance.\n'
  fi
    }
    ;;
  --browser-identity)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/mail"
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  LC_BROWSER_IDENTITY_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^(TestBrowserIdentityRealChain|TestBrowserSettingsWizardRealChain|TestMerchantAccountAPIProcessRestart)$' -v ./tests/foundation
  printf 'PASS: isolated PG + signed MOCK IdP browser chain; fixture removed at exit.\n'
    }
    ;;
  --browser-buyer-comms)
    lc_build=storefront
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/csvguard internal/customers internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/merchantorders internal/notify internal/promotions internal/returns internal/storefront internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # buyer-comms browser gate (independent R4 tests): refuse a no-test success before any build.
  test -f tests/foundation/browser_buyer_comms_test.go
  grep -q '^func TestBrowserBuyerComms' tests/foundation/browser_buyer_comms_test.go
  test -f tests/storefront/buyer-comms-gate.mjs
    }
    lc_run() {
  LC_BROWSER_BUYER_COMMS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run '^TestBrowserBuyerComms$' -v ./tests/foundation
  printf 'PASS: buyer-comms browser gate (BROWSER, MOCK mailbox): bank_transfer order with e-mail in the storefront shell, placed mail captured by a loopback SMTP fake, fresh-browser guest lookup (view-only, identical refusals); zh-TW + en, desktop + 390px; no real mailbox.\n'
    }
    ;;
  --browser-checkout-offline)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # COB (BROWSER, MOCK: no PSP on this path): refuse a no-test success before any build.
  test -f tests/foundation/browser_checkout_offline_test.go
  grep -q '^func TestBrowserCheckoutOffline' tests/foundation/browser_checkout_offline_test.go
  test -f tests/admin/checkout-offline.spec.ts
  test -f tests/storefront/offline-buyer.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CHECKOUT_OFFLINE_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserCheckoutOffline$' -v ./tests/foundation
  printf 'PASS: COB BROWSER (MOCK) isolated admin + storefront Next, Go, PG: merchant settings (bank details, window, free-shipping threshold) -> buyer home-delivery checkout with bank transfer + proof -> merchant confirm/reject in the order page -> buyer sees the result; the expiry function is run on a controlled clock and releases the stock; zh-TW + en, desktop + 390px; no PSP; not deployment acceptance.\n'
    }
    ;;
  --browser-home-cod)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/csvguard internal/customers internal/design internal/domains internal/fulfillment internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/promotions internal/reporting internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # home-cod R5 (BROWSER, MOCK: no PSP and no carrier API on this path): refuse a no-test success before any build.
  test -f tests/foundation/browser_home_cod_test.go
  grep -q '^func TestBrowserHomeCod' tests/foundation/browser_home_cod_test.go
  test -f tests/admin/home-cod.spec.ts
  test -f tests/storefront/home-cod-buyer.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_HOME_COD_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserHomeCod$' -v ./tests/foundation
  printf 'PASS: home-cod BROWSER (MOCK) isolated admin + storefront Next, Go, PG: merchant enables cash on delivery (cap, surcharge, carrier) -> buyer home-delivery checkout with cash on delivery -> merchant records the manual shipment and the collected cash -> buyer sees COLLECTED, finance shows the COD columns; zh-TW + en, desktop + 390px; no PSP and no carrier API; not deployment acceptance.\n'
    }
    ;;
  --browser-promotions)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/catalog internal/claims internal/csvguard internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/merchantorders internal/payments internal/pricing internal/promotions internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # promotions browser gate (independent R4 tests): refuse a no-test success before any build.
  test -f tests/foundation/browser_promotions_test.go
  grep -q '^func TestBrowserPromotions' tests/foundation/browser_promotions_test.go
  test -f tests/storefront/promotions-gate.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_PROMOTIONS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserPromotions$' -v ./tests/foundation
  printf 'PASS: promotions browser gate (BROWSER, MOCK IdP): the merchant creates a 10%% code with a minimum spend in /promotions, an anonymous buyer adds 2 products in the new storefront, applies the code at checkout, sees the discount and places a bank_transfer order, the merchant order view and finance show the discounted amounts; invalid and below-minimum codes show their messages; zh-TW + en, desktop + 390px; PG readback of codes/orders/redemptions; Stripe SANDBOX and WebKit are NOT_RUN here.\n'
    }
    ;;
  --browser-password-auth)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/billing internal/claims internal/csvguard internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/mail internal/merchantorders"
    lc_browsers=chromium
    lc_prepare() {
  # PA11: refuse a no-test success (merchant-password-auth-v1 §9).
  test -f tests/admin/password-auth.spec.ts
  test -f tests/admin/password-bff.test.ts
  test -f tests/foundation/browser_password_auth_test.go
  grep -q '^func TestBrowserPasswordAuth' tests/foundation/browser_password_auth_test.go
  # staff-team (R4) independent gate rides on the same loopback-SMTP harness: refuse a no-test success here too.
  test -f tests/admin/staff-team.spec.ts
  test -f tests/foundation/browser_staff_team_test.go
  grep -q '^func TestBrowserStaffTeam' tests/foundation/browser_staff_team_test.go
  mkdir -p output/playwright
    }
    lc_run() {
  node --test --experimental-strip-types tests/admin/password-bff.test.ts   # PA10 (Node, no browser, no PG)
  LC_BROWSER_PASSWORD_AUTH_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1800s -run '^(TestBrowserPasswordAuth|TestBrowserStaffTeam)$' -v ./tests/foundation
  printf 'PASS: isolated Next + Go + PG + loopback SMTP fake password-auth browser chain (PA11) and staff-team invite/accept/role/revoke chain (zh-TW + en, desktop + 390px); no real mailbox, no owner secret.\n'
    }
    ;;
  --browser-admin-legacy)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/catalog internal/mail"
    lc_browsers=chromium
    lc_prepare() {
  # Orphan-spec gate (ledger/production/visual-states + identity-mock + entry-mock): refuse a no-test success.
  test -f tests/foundation/browser_admin_legacy_test.go
  grep -q '^func TestBrowserAdminLedgerFixtureChain' tests/foundation/browser_admin_legacy_test.go
  grep -q '^func TestBrowserAdminIdentityMock' tests/foundation/browser_admin_legacy_test.go
  grep -q '^func TestBrowserAdminEntryMock' tests/foundation/browser_admin_legacy_test.go
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_ADMIN_LEGACY_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserAdmin(LedgerFixtureChain|IdentityMock|EntryMock)$' -v ./tests/foundation
  printf 'PASS: admin ledger (fixture bearer) + production fail-closed + identity-mock + entry-mock browser suites; no signed IdP, not production acceptance.\n'
    }
    ;;
  --browser-buyer)
    lc_build=storefront
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  LC_BROWSER_BUYER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserBuyerRealChain$' -v ./tests/foundation
  printf 'PASS: isolated PG + real buyer browser transport; not UI/PSP/deployment acceptance.\n'
    }
    ;;
  --browser-merchant-buyer)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/catalog internal/mail internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # Do not report a pass from an exact Go test selector matching no test.
  test -f tests/foundation/browser_merchant_buyer_chain_test.go
    }
    lc_run() {
  LC_BROWSER_MERCHANT_BUYER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserMerchantBuyerRealChain$' -v ./tests/foundation
  printf 'PASS: isolated merchant-to-buyer browser chain; not provider payment or real DNS/TLS deployment proof.\n'
    }
    ;;
  --browser-manual-order)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/buyer internal/buyerhttp internal/checkout internal/claims internal/domains internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/metaconnect internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_manual_order_link_test.go
  test -f tests/storefront/manual-order-link-gate.mjs
  node --test --experimental-strip-types apps/storefront/tests/order-link.test.mjs tests/admin/merchant-tools-model.test.ts
    }
    lc_run() {
  LC_BROWSER_MANUAL_ORDER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=300s -run '^TestBrowserManualOrderLink$' -v ./tests/foundation
  printf 'PASS: isolated merchant UI manual order -> buyer link -> fresh browser exchange -> bank details -> transfer proof; signed MOCK IdP, local TLS/CONNECT edge; not provider or deployment acceptance.\n'
    }
    ;;
  --browser-merchant-orders-bff)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/catalog internal/claims internal/csvguard internal/customers internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefront internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_merchant_orders_bff_test.go
  test -f tests/admin/orders-bff.spec.ts
  # Raw URL grammar is shared by Proxy and route; real HTTP below additionally
  # proves that the framework cannot normalize a rejected request past it.
  node --test --experimental-strip-types tests/admin/orders-request.test.ts
  # W3-07B: parcel route grammar (DELETE dissolve + merge-suggestions read + keyed commands).
  node --test --experimental-strip-types tests/admin/parcels-request.test.ts
  # W3-U4: the REAL route handler with a stub Go upstream: bodyless dissolve DELETE (Next 16 empty body stream), CAS query on every
  # DELETE, GET parcel-groups grammar (node:module registerHooks, no browser/PG).
  node --test --experimental-strip-types tests/admin/parcels-bff.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_MERCHANT_ORDERS_BFF_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserMerchantOrdersBFFRealChain$' -v ./tests/foundation
  printf 'PASS: isolated Next + Go + PG merchant-order read transport; not merchant UI or provider acceptance.\n'
    }
    ;;
  --browser-merchant-orders-ui)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/csvguard internal/customers internal/design internal/domains internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefront internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_merchant_orders_ui_test.go
  test -f tests/admin/orders-ui.spec.ts
  # W3-07B parcel-merge real-click spec (runs after orders-ui.spec inside the Go harness).
  test -f tests/admin/parcel-merge.spec.ts
  # Pure model contract behind the UI (orders/refunds/shipments parsers); was run by no gate before.
  node --test --experimental-strip-types tests/admin/orders-model.test.ts
  # checkout-offline (storefront-v2 §C): bank-transfer DTO parsers, copy parity and the finance row shape.
  node --test --experimental-strip-types tests/admin/transfer-model.test.ts
  # storefront-publish R3 (base defect found by check-gates: this tracked file was run by no gate).
  node --test --experimental-strip-types tests/admin/storefront-model.test.ts
  # W3-07B: parcel DTO parsers + three-locale copy parity (incl. the owner ruling sentence and refusal codes).
  node --test --experimental-strip-types tests/admin/parcels-model.test.ts
  # W3-07B: BFF parcel route grammar + dissolve query validator (shared with the --browser-merchant-orders-bff gate).
  node --test --experimental-strip-types tests/admin/parcels-request.test.ts tests/admin/parcels-bff.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  # Native tab focus/visibility assertions require headed Chromium; keep them intact under CI's virtual X server.
  mou_browser_command=(go)
  if [[ "$(uname -s)" == Linux && -z "${DISPLAY:-}" ]]; then
    command -v xvfb-run >/dev/null || { printf 'MOU: headed native checks require xvfb-run on Linux without DISPLAY\n' >&2; exit 2; }
    mou_browser_command=(xvfb-run --auto-servernum go)
  fi
  LC_BROWSER_MERCHANT_ORDERS_UI_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 "${mou_browser_command[@]}" test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserMerchantOrdersUIRealChain$' -v ./tests/foundation
  printf 'PASS: isolated merchant C order UI + W3-07B parcel-merge UI; signed MOCK IdP and local payment fixtures, not production/provider acceptance.\n'
    }
    ;;
  --browser-input-delivery)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/claims internal/inbox internal/live internal/mail"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_input_delivery_test.go
  test -f tests/admin/input-delivery.spec.ts
  grep -q '^func TestBrowserInputDeliveryBRW05RealChain' tests/foundation/browser_input_delivery_test.go
  node --test --experimental-strip-types tests/admin/studio-request.test.ts tests/admin/studio-input.test.ts
  # Runs the actual browser client (including parameter-property syntax) under
  # pinned Node 24, before the separate signed HTTPS/PG transport fixture.
  node --test --experimental-transform-types tests/admin/studio-input-client.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_INPUT_DELIVERY_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=240s -run '^TestBrowserInputDeliveryBRW05RealChain$' -v ./tests/foundation
  printf 'PASS: isolated HTTPS signed browser + Next + Go + PG input token transport; not decoded SFU media, recovery or production acceptance.\n'
    }
    ;;
  --browser-studio-bff)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/catalog internal/claims internal/csvguard internal/customers internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/admin/studio-request.test.ts
  test -f tests/admin/studio-bff.spec.ts
  test -f tests/foundation/browser_studio_bff_test.go
  grep -q '^func TestBrowserStudioBFFRealChain' tests/foundation/browser_studio_bff_test.go
  node --test --experimental-strip-types tests/admin/studio-request.test.ts tests/admin/studio-input.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_STUDIO_BFF_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=240s -run '^TestBrowserStudioBFFRealChain$' -v ./tests/foundation
  printf 'PASS: isolated signed OIDC + Next + Go + PG Studio BFF transport; not Studio page/UI, Cloud or provider acceptance.\n'
    }
    ;;
  --browser-studio-ui)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/live internal/mail"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/admin/studio-ui.spec.ts
  test -f tests/foundation/browser_studio_ui_test.go
  grep -q '^func TestBrowserStudioUIRealChain' tests/foundation/browser_studio_ui_test.go
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_STUDIO_UI_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=360s -run '^TestBrowserStudioUIRealChain$' -v ./tests/foundation
  printf 'PASS: isolated Studio B UI with signed MOCK IdP and local MOCK Egress; not Cloud or production acceptance.\n'
    }
    ;;
  --browser-live-console)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/integrations/metareply internal/inventory internal/live internal/mail internal/metaconnect internal/msgtemplates"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/admin/live-console.spec.ts
  test -f tests/foundation/browser_live_console_test.go
  grep -q '^func TestBrowserLiveConsoleRealChain' tests/foundation/browser_live_console_test.go
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=780s -run '^TestBrowserLiveConsoleRealChain$' -v ./tests/foundation
  printf 'PASS: LC-U1 MOCK Console and LC-U2a REAL_PG comment/inbox browser; MOCK Graph only, no LIVE provider acceptance.\n'
    }
    ;;
  --browser-claim-checkout)
    lc_build=storefront
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/checkout internal/claims internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/storefront/claim-checkout.mjs
  test -f tests/foundation/browser_claim_checkout_test.go
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CLAIM_CHECKOUT_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=900s -run '^TestBrowserClaimDirectCheckout$' -v ./tests/foundation
  printf 'PASS: claim direct checkout real-click BROWSER + MOCK manual claims, real PG; no PSP/Meta/LIVE acceptance.\n'
    }
    ;;
  --browser-live-claims)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/live internal/mail internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # KC16: refuse a no-test success, and run the pure claims BFF/copy contracts first.
  test -f tests/admin/claims-ui.spec.ts
  test -f tests/foundation/browser_live_claims_test.go
  grep -q '^func TestBrowserLiveClaimsRealChain' tests/foundation/browser_live_claims_test.go
  node --test --experimental-strip-types tests/admin/claims-request.test.ts tests/admin/claim-source.test.ts tests/admin/claims-model.test.ts apps/storefront/tests/claim.test.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_LIVE_CLAIMS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=360s -run '^TestBrowserLiveClaimsRealChain$' -v ./tests/foundation
  printf 'PASS: KC16 isolated admin + storefront Next, Go and PG claims chain; signed MOCK IdP, MOCK manual ingress; no provider or deployment acceptance.\n'
    }
    ;;
  --browser-order)
    lc_build=storefront
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_order_chain_test.go
    }
    lc_run() {
  LC_BROWSER_ORDER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserBuyerOrderUI$' -v ./tests/foundation
  printf 'PASS: isolated buyer address/order UI gate; not provider payment or deployment acceptance.\n'
    }
    ;;
  --browser-payment)
    lc_build=storefront
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_payment_chain_test.go
    }
    lc_run() {
  LC_BROWSER_PAYMENT_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=600s -run '^TestBrowserBuyerPaymentUI$' -v ./tests/foundation
  printf 'PASS: isolated buyer payment UI/native POST gate with a local mock PSP; not provider payment or deployment acceptance.\n'
    }
    ;;
  --stripe-browser)
    lc_build=storefront
    lc_fixture=pg
    lc_browsers=chromium
    lc_prepare() {
  # SP18 / SU05-SU09 (contracts/stripe-buyer-ui-v1.md §9). Steps can be narrowed with
  # LC_STRIPE_BROWSER_STEPS (default: all); only sp18/su07/su09 need the SANDBOX variables.
  test -f tests/foundation/browser_stripe_test.go
  test -f tests/storefront/stripe-browser.mjs
  test -f tests/storefront/payuni-ui-baseline.mjs
  stripe_steps=",${LC_STRIPE_BROWSER_STEPS:-node-env,payuni-baseline,sp18,su07,su09},"
  stripe_key=""
  if [[ "$stripe_steps" == *,sp18,* || "$stripe_steps" == *,su07,* || "$stripe_steps" == *,su09,* || "$stripe_steps" == *,obs,* ]]; then
    if [[ "${STRIPE_BROWSER:-}" != 1 || "${STRIPE_SANDBOX:-}" != 1 ]]; then
      printf 'NOT_RUN: SP18 requires STRIPE_BROWSER=1 and STRIPE_SANDBOX=1 (Stripe test-mode key from secrets.env, account acct_1UJDb0RusP6Wwj7e); nothing was started.\n' >&2
      exit 2
    fi
    # Only STRIPE_SECRET_KEY is read, in a subshell (never `set -a` the file, never echoed).
    stripe_secrets="${LC_SECRETS_FILE:-$HOME/.config/livecommerce/secrets.env}"
    stripe_key="$(set +x; . "$stripe_secrets" 2>/dev/null; printf %s "${STRIPE_SECRET_KEY:-}")"
    if [[ ! "$stripe_key" =~ ^(sk|rk)_test_ ]]; then
      printf 'NOT_RUN: STRIPE_SECRET_KEY in secrets.env is missing or not a Stripe test key (^(sk|rk)_test_); nothing was started.\n' >&2
      exit 2
    fi
    stripe_account="${STRIPE_ACCOUNT_ID:-acct_1UJDb0RusP6Wwj7e}"
    if [[ "$stripe_account" != acct_1UJDb0RusP6Wwj7e ]]; then
      printf 'NOT_RUN: STRIPE_ACCOUNT_ID must be the SANDBOX fixture account (matches SP16).\n' >&2
      exit 2
    fi
    printf 'SANDBOX checkout.stripe.com test mode; no live charge; SP17 webhook NOT_RUN\n'
  fi
    }
    lc_run() {
  # Durable evidence is selected once in the shared run-root prelude.
  stripe_out="$LC_BROWSER_EVIDENCE_ROOT/stripe-b2-browser-tests"
  mkdir -p "$stripe_out"
  stripe_sha="$(git rev-parse --short=12 HEAD)"
  stripe_status=0
  # Every go test process needs a FRESH cluster: the foundation fixture creates cluster-scoped roles
  # (foundation_api, ...) and refuses a cluster that already has them. Recreate the task-owned container
  # (same pinned image, limits and loopback binding as above) before each step.
  stripe_fresh_pg() { fresh_pg; }
  # run_stripe_step <label> <go -run regex> <timeout> <min leaf cases> [env assignments...]
  # Parses `go test -json` (contract §14 parsing rule): any SKIP, any FAIL, fewer leaf cases than
  # expected, zero passing tests, a missing log or a non-zero exit is FAIL, never PASS.
  run_stripe_step() {
    local label="$1" regex="$2" tmo="$3" min="$4"; shift 4
    local log="$stripe_out/$stripe_sha-$label.jsonl" started rc=0 verdict=0 counts
    stripe_fresh_pg
    started="$(date +%s)"
    # Only this go test process (never Node) receives the Stripe key; the Go test strips it again.
    env "$@" LC_STRIPE_BROWSER_ACCEPTANCE=1 LC_STRIPE_EVIDENCE_ROOT="$stripe_out" LC_BASELINE_OUT_DIR="$stripe_out" \
      GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout="$tmo" -json -run "$regex" ./tests/foundation >"$log" 2>"$log.stderr" || rc=$?
    counts="$(go_json_counts "$log" "$min")"
    verdict="${counts##*VERDICT=}"; counts="${counts%% VERDICT=*}"
    printf '%s: %s exit=%d verdict=%d duration=%ds log=%s\n' "$label" "$counts" "$rc" "$verdict" "$(( $(date +%s) - started ))" "$log"
    if [[ "$rc" != 0 || "$verdict" != 0 ]]; then stripe_status=1; printf 'FAIL: %s (go test exit=%d, parse verdict=%d)\n' "$label" "$rc" "$verdict" >&2; fi
  }
  sandbox_env=(STRIPE_BROWSER=1 STRIPE_SANDBOX=1 STRIPE_ACCOUNT_ID="${stripe_account:-}" STRIPE_SECRET_KEY="$stripe_key")
  if [[ "$stripe_steps" == *,node-env,* ]]; then
    run_stripe_step node-env '^TestBrowserStripeNodeEnv$' 120s 0
  fi
  if [[ "$stripe_steps" == *,payuni-baseline,* ]]; then
    run_stripe_step payuni-baseline '^TestBrowserPayuniBaseline$' 300s 0 LC_BASELINE_SHA="$stripe_sha" LC_BASELINE_MUTATE="${LC_BASELINE_MUTATE:-}"
  fi
  if [[ "$stripe_steps" == *,sp18,* ]]; then
    if [[ "${STRIPE_BROWSER_SPLIT:-}" == 1 ]]; then
      for scenario in A B C; do run_stripe_step "sp18-$scenario" "^TestBrowserStripeCheckout\$/^SP18$scenario" 900s 2 "${sandbox_env[@]}"; done
    else
      run_stripe_step sp18 '^TestBrowserStripeCheckout$/^SP18' 900s 6 "${sandbox_env[@]}"
    fi
  fi
  if [[ "$stripe_steps" == *,obs,* ]]; then
    # Developer-only (not in the default steps): real hosted page observed without the B2 buyer UI.
    run_stripe_step obs "^TestBrowserStripeCheckout\$/^${LC_STRIPE_OBS:-OBS}" 900s 1 "${sandbox_env[@]}" STRIPE_BROWSER_OBSERVE=1
  fi
  if [[ "$stripe_steps" == *,su07,* ]]; then run_stripe_step su07 '^TestBrowserStripeCheckout$/^SU07' 900s 1 "${sandbox_env[@]}"; fi
  if [[ "$stripe_steps" == *,su09,* ]]; then run_stripe_step su09 '^TestBrowserStripeCheckout$/^SU09' 900s 2 "${sandbox_env[@]}"; fi
  if [[ "$stripe_status" != 0 ]]; then printf 'FAIL: --stripe-browser (see logs in %s)\n' "$stripe_out" >&2; exit 1; fi
  printf 'PASS: stripe-browser steps [%s] (SP18 = SANDBOX, SU07/SU09 = MOCK, SU05 baseline capture).\n' "${stripe_steps//,/ }"
  if [[ -n "$stripe_key" ]]; then printf 'SANDBOX checkout.stripe.com test mode; no live charge; SP17 webhook NOT_RUN\n'; fi
    }
    ;;
  --browser-refund-fulfilment)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/checkout internal/claims internal/csvguard internal/domains internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/merchantorders internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # MF07 + RF11 (BROWSER, MOCK Stripe): refuse a no-test success and run the pure BFF model gate first.
  test -f tests/foundation/browser_refund_fulfilment_test.go
  grep -q '^func TestBrowserManualFulfilment' tests/foundation/browser_refund_fulfilment_test.go
  grep -q '^func TestBrowserRefund' tests/foundation/browser_refund_fulfilment_test.go
  test -f tests/admin/manual-fulfilment.spec.ts
  test -f tests/admin/refund.spec.ts
  test -f tests/storefront/shipment-buyer.mjs
  test -f tests/storefront/refund-buyer.mjs
  node --test --experimental-strip-types tests/admin/refund-bff.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_REFUND_FULFILMENT_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run '^(TestBrowserManualFulfilment|TestBrowserRefund)$' -v ./tests/foundation
  printf 'PASS: MF07 + RF11(a) isolated admin + storefront Next, Go, PG, real worker and the MOCK Stripe fake; RF11(b) SANDBOX is NOT_RUN unless it says otherwise above; not provider or deployment acceptance.\n'
    }
    ;;
  --browser-customers-billing)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/attribution internal/billing internal/billing/billingtest internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/csvguard internal/customers internal/design internal/domains internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefront internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # CB11 (BROWSER, MOCK Stripe): refuse a no-test success and run the pure BFF fence/decoder gate first.
  test -f tests/foundation/browser_customers_billing_test.go
  grep -q '^func TestBrowserCustomersBilling' tests/foundation/browser_customers_billing_test.go
  test -f tests/admin/customers-billing.spec.ts
  test -f tests/foundation/browser_w6_customers_reports_test.go
  grep -q '^func TestBrowserW6Customers' tests/foundation/browser_w6_customers_reports_test.go
  test -f tests/admin/customer-tags.spec.ts
  test -f tests/storefront/privacy-buyer.mjs
  node --test --experimental-strip-types tests/admin/customers-bff.test.ts tests/admin/customers-model.test.ts \
    tests/admin/customers-request.test.ts tests/admin/billing-model.test.ts tests/admin/customer-tags-bff.test.ts tests/admin/customer-tags-proxy.test.ts tests/admin/customer-tags-client.test.mjs tests/admin/customer-tags-draft.test.mjs tests/admin/customer-tags-write.test.mjs tests/admin/w6-integration.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CUSTOMERS_BILLING_ACCEPTANCE=1 LC_W6UI_CUSTOMERS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=2400s -run '^(TestBrowserCustomersBilling|TestBrowserW6Customers)$' -v ./tests/foundation
  printf 'PASS: CB11 isolated admin + storefront Next, Go, PG, real worker; platform billing = MOCK (independent billingtest fake, Stripe pages answered in the browser); not provider or deployment acceptance; CB10 SANDBOX and CB12 LIVE are NOT_RUN.\n'
    }
    ;;
  --browser-card-payments)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/csvguard internal/customers internal/design internal/domains internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefront internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # W4-U1 (BROWSER, MOCK Stripe): refuse a no-test success and run the pure parser / BFF-fence / copy / wire-fixture / state-machine gates first.
  test -f tests/foundation/browser_card_payments_test.go
  grep -q '^func TestBrowserCardPayments' tests/foundation/browser_card_payments_test.go
  test -f tests/foundation/w4_u1_seed_test.go
  test -f tests/admin/card-payments.spec.ts
  test -f tests/admin/settlements.spec.ts
  test -f tests/storefront/collector-buyer.mjs
  node --test --experimental-strip-types tests/admin/card-payments-model.test.ts tests/admin/card-payments-request.test.ts \
    tests/admin/card-payments-copy.test.ts tests/admin/card-payments-wire.test.ts apps/storefront/tests/payment-collector.test.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CARD_PAYMENTS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=2400s -run '^TestBrowserCardPayments$' -v ./tests/foundation
  printf 'PASS: W4-U1 isolated admin + storefront Next, Go, PG, real worker; platform Stripe = MOCK (independent fake; designate/open/allowlist/block through the operator definers; statements through sync -> close -> payout record), no Stripe, no key, test mode only; merchant enable/disable (CAS PUT, 409 reload), every state badge, platform-not-OPEN hides the toggle, BLOCKED disable-only, settlements list + detail, no acct_/sk_/rk_ in DOM/network/storage; buyer collector disclosure in zh-TW + zh-CN + en above the pay button and on the paid page, absent for a primary connection; desktop + 390px; not Stripe, provider or deployment acceptance; SANDBOX and LIVE are NOT_RUN.\n'
    }
    ;;
  --browser-meta-ads)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/attribution internal/billing internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/csvguard internal/customers internal/design internal/domains internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefront internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # MA09a (BROWSER, Meta = MOCK): refuse a no-test success and run the pure model/request gates first.
  test -f tests/foundation/browser_meta_ads_test.go
  grep -q '^func TestBrowserMetaAds' tests/foundation/browser_meta_ads_test.go
  grep -q '^func TestBrowserMetaAdsConsent' tests/foundation/browser_meta_ads_test.go
  test -f tests/admin/ads.spec.ts
  test -f tests/storefront/ads-consent.mjs
  node --test --experimental-strip-types tests/admin/ads-model.test.ts tests/admin/ads-request.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  # AL1: the frozen ad link / feed link (origin + /products/{id}) must reach a 200 page on the production storefront build.
  node tests/storefront/ad-link.mjs
  node --test --experimental-strip-types apps/storefront/tests/ad-link-route.test.mjs
  LC_BROWSER_META_ADS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserMetaAds(Consent)?$' -v ./tests/foundation
  printf 'PASS: MA09a isolated admin Next, Go API + ads worker, PG; Meta = MOCK (fake Graph + the Facebook Login dialog answered by the browser route); not Meta, provider or deployment acceptance; MA09b buyer consent -> CAPI context runs against the production storefront Next build.\n'
    }
    ;;
  --browser-ads-attribution)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/attribution internal/buyer internal/buyerhttp internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_ads_attribution_test.go
  grep -q '^func TestBrowserAdsAttribution' tests/foundation/browser_ads_attribution_test.go
  test -f tests/admin/attribution.spec.ts
  test -f tests/storefront/ads-attribution.mjs
  node --test --experimental-strip-types tests/admin/attribution.test.ts tests/admin/attribution-audience.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_ATTRIBUTION_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=2400s -run '^TestBrowserAdsAttribution' -v ./tests/foundation
  printf 'PASS: AT5/AT9 real-click storefront and authenticated report against isolated PG/Go and production Next builds; Meta=MOCK only; no live/sandbox acceptance.\n'
    }
    ;;
  --browser-cvs)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/claims internal/fulfillment internal/integrations/shipping/ecpay internal/integrations/shipping/ecpay/ecpaytest internal/mail"
    lc_browsers="chromium webkit"
    lc_prepare() {
  # TCV08 (BROWSER, MOCK ECPay map/Create + signed status posts): refuse a no-test success before any build.
  test -f tests/foundation/browser_taiwan_cvs_test.go
  grep -q '^func TestBrowserTaiwanCvs' tests/foundation/browser_taiwan_cvs_test.go
  test -f tests/admin/taiwan-cvs.spec.ts
  test -f tests/storefront/cvs-buyer.mjs
  # Pure admin BFF grammar/model gates first (LGR/LGM): no Docker needed, fail before any build.
  node --test --experimental-strip-types tests/admin/logistics-model.test.ts tests/admin/logistics-request.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CVS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserTaiwanCvs$' -v ./tests/foundation
  printf 'PASS: TCV08 MOCK isolated admin + storefront Next, Go, PG, ecpaytest fake map/Create and signed status posts; SANDBOX and WebKit variants are NOT_RUN unless the go test log says otherwise; not provider or deployment acceptance.\n'
    }
    ;;
  --browser-catalog-media)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/buyer internal/buyerhttp internal/catalog internal/claims internal/csvguard internal/customers internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # catalog-media browser gate (independent R3 tests): refuse a no-test success before any build.
  test -f tests/foundation/browser_catalog_media_test.go
  grep -q '^func TestBrowserCatalogMedia' tests/foundation/browser_catalog_media_test.go
  test -f tests/storefront/catalog-media-gate.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CATALOG_MEDIA_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run '^TestBrowserCatalogMedia$' -v ./tests/foundation
  printf 'PASS: catalog-media browser gate (BROWSER, MOCK IdP): merchant uploads 2 photos, reorders, renames, reprices and archives a SKU in the product editor and reads the results back in the Ledger; an anonymous buyer sees the home grid, gallery order, new price and no archived SKU; zh-TW + en, desktop + 390px; the storefront publication and the ACTIVE domain are written through the migration 0081 definers (merchant publish, operator bind), not owner-seeded; not provider or deployment acceptance.\n'
    }
    ;;
  --browser-storefront-publish)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/buyer internal/buyerhttp internal/claims internal/design internal/mail internal/metaconnect internal/storefront internal/storefrontadmin internal/storehandles"
    lc_browsers=chromium
    lc_prepare() {
  # R3 storefront-publish KEY gate (BROWSER, MOCK edge): refuse a no-test success and run the pure card-model gate first.
  test -f tests/foundation/browser_storefront_publish_test.go
  grep -q '^func TestBrowserStorefrontPublish' tests/foundation/browser_storefront_publish_test.go
  test -f tests/storefront/storefront-publish-gate.mjs
  test -f tests/admin/design.spec.ts
  node --test --experimental-strip-types tests/admin/storefront-model.test.ts tests/admin/design-gate.test.ts tests/admin/design-model.test.ts packages/markdown-lite/tests/index.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_STOREFRONT_PUBLISH_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=900s -run '^TestBrowserStorefrontPublish$' -v ./tests/foundation
  printf 'PASS: R3 storefront-publish: isolated admin + storefront Next, Go, PG with NO owner-seeded publication/domain row; the merchant publishes with the Settings card (en/zh-TW, desktop + 390px), the built cmd/store-admin executable binds/suspends/detaches/re-binds the origin, a fresh anonymous buyer browser sees the product or the not-found page on https://buyer.example; signed MOCK IdP + synthetic TLS/CONNECT edge, ownership/TLS evidence is an unverified reference; not DNS/TLS, Caddy or provider acceptance.\n'
    }
    ;;
  --browser-store-domains)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/buyer internal/buyerhttp internal/catalog internal/claims internal/csvguard internal/customers internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # R5 store-domains KEY gate (BROWSER, MOCK edge + MOCK DNS/TLS): refuse a no-test success and run the pure domain-model gate first.
  test -f tests/foundation/browser_store_domains_test.go
  grep -q '^func TestBrowserStoreDomains' tests/foundation/browser_store_domains_test.go
  test -f tests/storefront/store-domains-gate.mjs
  node --test --experimental-strip-types tests/admin/storefront-model.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_STORE_DOMAINS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1200s -run '^TestBrowserStoreDomains$' -v ./tests/foundation
  printf 'PASS: R5 store-domains: isolated admin + storefront Next, Go, PG with NO owner-seeded domain row; the onboarding wizard shows the server-assigned random eight-digit address only after creation, the merchant publishes and the storefront is served at https://<handle>.<base> through the synthetic Host-mapped edge, the merchant adds a custom domain, sees the DNS instructions, the MOCK DNS+TLS turn green -> ACTIVE and the platform subdomain 301s to it; zh-TW + zh-CN + en, desktop + 390px; signed MOCK IdP, MOCK DNS/TLS edge: not real DNS/TLS, Caddy on_demand_tls or provider acceptance.\n'
    }
    ;;
  --browser-storefront)
    lc_build=storefront
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/design internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/mail internal/merchanttools internal/storefront internal/storefrontadmin internal/storehandles"
    lc_browsers=chromium
    lc_prepare() {
  # Real stack (unit storefront-integration): the pure logic tests first (the MOCK fake only feeds node tests/the LC_SHOP_MOCK variant), then PG + Go + Next.
  test -f tests/foundation/browser_storefront_test.go
  grep -q '^func TestBrowserStorefront' tests/foundation/browser_storefront_test.go
  test -f tests/storefront/shop-real-gate.mjs
  node --test --experimental-strip-types apps/storefront/tests/shop.test.mjs
    }
    lc_run() {
  # SFR gate (unit storefront-integration, SANDBOX): the storefront shell on the production build against the real buyerhttp handler and PG,
  # a shop built through the real admin API and published through the 0081 definers (tests/foundation/browser_storefront_test.go).
  LC_BROWSER_STOREFRONT_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run '^TestBrowserStorefront$' -v ./tests/foundation
  printf 'PASS: SFR01-SFR09 buyer storefront shell on the REAL stack (production Next + real Go buyer API + isolated PG; shop built through the admin API, published through the migration 0081 definers; checkout path, free-shipping threshold and collection photo ids from the producers); synthetic CONNECT edge, no provider, no device. The MOCK sibling: LC_SHOP_MOCK=1 bash scripts/dev/test-local.sh --browser-storefront.\n'
    }
    if [[ "${LC_SHOP_MOCK:-0}" == 1 ]]; then
      lc_build=none
      lc_fixture=none
      lc_prepare() { :; }
      lc_run() {
  # LC_SHOP_MOCK=1: the fast MOCK variant (no PG, no Docker). The default --browser-storefront is the REAL stack (below, unit storefront-integration).
  # SF gate (unit storefront-shell, MOCK tier): the buyer storefront shell on the production Next build against a contract-shaped FAKE of the
  # Go buyer API (tests/storefront/shop-fake-api.mjs). No PG/Go/Docker is started, so it needs no machine-wide PG lock. Not real-stack acceptance:
  # run the same pages against the real API once catalog-core (0086) and store-design (0087) are merged (docs/delivery/GATES.md).
  command -v pnpm >/dev/null
  command -v openssl >/dev/null
  test -f tests/storefront/shop-gate.mjs
  test -f tests/storefront/shop-fake-api.mjs
  node --test --experimental-strip-types apps/storefront/tests/shop.test.mjs
  COMMERCE_BUYER_WEB_ENABLED=0 pnpm run build:storefront
  shop_out="${LC_SHOP_EVIDENCE:-$LC_BROWSER_EVIDENCE_ROOT/storefront-shell}"
  mkdir -p "$shop_out"
  LC_SHOP_EVIDENCE="$shop_out" node tests/storefront/shop-gate.mjs | tee "$shop_out/shop-gate.log"
  grep -q 'cases=12 ' "$shop_out/shop-gate.log"
  printf 'PASS: SF01-SF12 MOCK buyer storefront shell (SF11 = the visual-QA polish checks) (engine per LC_BROWSER_ENGINE) on the production Next build + contract-shaped fake API; not real Go/PG, published-origin resolver, order placement or deployment acceptance. Log: %s\n' "$shop_out/shop-gate.log"
  exit 0
      }
    fi
    ;;
  --browser-webkit)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/attribution internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/customers internal/design internal/domains internal/fulfillment internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/integrations/shipping/ecpay/ecpaytest internal/jobqueue internal/mail internal/promotions internal/storefront internal/storefrontdomains internal/twcity"
    lc_browsers="chromium webkit"
    lc_prepare() {
  # WebKit/iPhone Safari coverage of the buyer-critical flows (MOCK tier; docs/delivery/GATES.md). Refuse before any build when the WebKit
  # browser is absent: that is NOT_RUN (exit 2), never a green run on Chromium.
  test -f tests/storefront/browser-engine.mjs
  command -v node >/dev/null
  if ! node --input-type=module -e 'import { webkit } from "@playwright/test"; import { existsSync } from "node:fs"; process.exit(existsSync(webkit.executablePath()) ? 0 : 1)' 2>/dev/null; then
    printf 'NOT_RUN: Playwright WebKit is not installed (pnpm exec playwright install webkit); nothing was started.\n' >&2
    exit 2
  fi
  mkdir -p output/playwright
    }
    lc_run() {
  # Same production storefront + admin builds and the same go tests as the Chromium modes, with LC_BROWSER_ENGINE=webkit: phone-sized buyer
  # contexts run Playwright's iPhone 15 profile, desktop ones and every admin page run Desktop Safari. One go test process per step on a
  # fresh PG cluster; any SKIP/FAIL, too few leaf cases or a missing log is a failed step (go_json_counts). Evidence stays in the selected run root.
  webkit_out="$LC_BROWSER_EVIDENCE_ROOT/webkit"
  mkdir -p "$webkit_out"
  webkit_sha="$(git rev-parse --short=12 HEAD)"
  webkit_status=0
  # Steps can be narrowed while fixing one flow (LC_WEBKIT_STEPS=payment,cvs); the default is all seven and release-gate.sh requires all seven.
  webkit_steps=",${LC_WEBKIT_STEPS:-buyer,order,payment,merchant-buyer,cvs,password-auth,claim-checkout},"
  # run_webkit_step <label> <acceptance env var> <go -run regex> <timeout> <min leaf cases>
  run_webkit_step() {
    local label="$1" accept="$2" regex="$3" tmo="$4" min="$5"
    [[ "$webkit_steps" == *",$label,"* ]] || return 0
    local log="$webkit_out/$webkit_sha-$label.jsonl" started rc=0 verdict=0 counts
    fresh_pg
    started="$(date +%s)"
    env "$accept=1" LC_BROWSER_ENGINE=webkit GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout="$tmo" -json -run "$regex" ./tests/foundation >"$log" 2>"$log.stderr" || rc=$?
    counts="$(go_json_counts "$log" "$min")"
    verdict="${counts##*VERDICT=}"; counts="${counts%% VERDICT=*}"
    printf '%s: %s exit=%d verdict=%d duration=%ds log=%s\n' "$label" "$counts" "$rc" "$verdict" "$(( $(date +%s) - started ))" "$log"
    if [[ "$rc" != 0 || "$verdict" != 0 ]]; then webkit_status=1; printf 'FAIL: %s (go test exit=%d, parse verdict=%d)\n' "$label" "$rc" "$verdict" >&2; fi
  }
  run_webkit_step buyer LC_BROWSER_BUYER_ACCEPTANCE '^TestBrowserBuyerRealChain$' 900s 0
  run_webkit_step order LC_BROWSER_ORDER_ACCEPTANCE '^TestBrowserBuyerOrderUI$' 900s 0
  run_webkit_step payment LC_BROWSER_PAYMENT_ACCEPTANCE '^TestBrowserBuyerPaymentUI$' 900s 0
  run_webkit_step merchant-buyer LC_BROWSER_MERCHANT_BUYER_ACCEPTANCE '^TestBrowserMerchantBuyerRealChain$' 900s 0
  run_webkit_step cvs LC_BROWSER_CVS_ACCEPTANCE '^TestBrowserTaiwanCvs$/^WebKit$' 1700s 1
  run_webkit_step password-auth LC_BROWSER_PASSWORD_AUTH_ACCEPTANCE '^TestBrowserPasswordAuth$' 1200s 0
  run_webkit_step claim-checkout LC_BROWSER_CLAIM_CHECKOUT_ACCEPTANCE '^TestBrowserClaimDirectCheckout$' 900s 1
  if [[ "$webkit_status" != 0 ]]; then printf 'FAIL: --browser-webkit (see logs in %s)\n' "$webkit_out" >&2; exit 1; fi
  printf 'PASS: --browser-webkit MOCK tier on Playwright WebKit (iPhone 15 buyer, Desktop Safari admin behind a self-signed https front) steps [%s]: buyer, order, payment, merchant-buyer, cvs (TCV08 buyer + merchant), password-auth, claim-checkout; Stripe SP18 SANDBOX on WebKit = LC_BROWSER_ENGINE=webkit --stripe-browser (see GATES.md); not provider, real-device or deployment acceptance.\n' "${webkit_steps//,/ }"
    }
    ;;
  --browser-e2e)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/catalog internal/checkout internal/claims internal/claimsintake internal/csvguard internal/domains internal/fulfillment internal/integrations/accounts internal/integrations/meta internal/integrations/metareply internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/mail internal/merchantorders internal/payments internal/payments/stripeadmin internal/pricing internal/promotions internal/returns internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # T12: refuse a no-test success; the whole deal loop is one Go test driving one Playwright spec.
  test -f tests/foundation/browser_e2e_test.go
  grep -q '^func TestBrowserE2EDealLoop' tests/foundation/browser_e2e_test.go
  test -f tests/e2e/deal-loop.spec.ts
  # live-tools (R4) independent gate: runs in this mode after the deal loop (same builds, same fixture rules).
  test -f tests/foundation/browser_live_tools_test.go
  grep -q '^func TestBrowserLiveTools' tests/foundation/browser_live_tools_test.go
  test -f tests/e2e/live-tools.spec.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_E2E_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run "${LC_E2E_RUN:-^(TestBrowserE2EDealLoop(Sandbox)?|TestBrowserLiveTools)$}" -v ./tests/foundation
  printf 'PASS: T12 isolated admin + storefront Next, Go, PG, real worker/consumer/poller/dispatcher; Meta = MOCK (signed webhook, fake Graph), Stripe = MOCK (fake + routed hosted page); SANDBOX variant NOT_RUN unless it says otherwise above; not provider or deployment acceptance. Live tools (R4): Studio library import + live price, signed MOCK Meta claim, pay at pickup at the live price, direct purchase at the normal price (zh-TW + en, desktop + 390 px).\n'
    }
    ;;
  --checkout)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  # Focused diagnosis uses the same isolated real PG and cleanup guard. It never
  # substitutes for the full foundation/race/vet release gate below.
  # This selector now includes the expiry-worker crash/rescue suites as well as
  # checkout. Their aggregate exceeded 120s; individual SQL/deadline gates stay.
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=240s -run '^TestBuyerCheckout' -v ./tests/foundation
  printf 'PASS: checkout subset only; full regression still required.\n'
    }
    ;;
  --payment)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestBuyerPayment' -v ./tests/foundation
  printf 'PASS: payment start/query subset only; full regression still required.\n'
    }
    ;;
  --payment-worker)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/payment_runtime_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=120s -run '^TestBuyerPaymentWorker' -v ./tests/foundation
  printf 'PASS: isolated payment worker subset; no real-provider or deployment claim.\n'
    }
    ;;
  --expiry-worker)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/expiry_runtime_test.go
  test -f tests/foundation/expiry_admission_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=150s -run '^TestBuyerCheckoutExpiryRuntime' -v ./tests/foundation
  printf 'PASS: isolated expiry worker subset; no production or recovery-SLO claim.\n'
    }
    ;;
  --storefront-resolver)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=120s -run '^TestPublishedStorefront' -v ./tests/foundation
  printf 'PASS: published-origin resolver subset only; not public HTTP or provider proof.\n'
    }
    ;;
  --buyer-http)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=120s -run '^TestBuyerHTTP' -v ./tests/foundation
  printf 'PASS: private buyer HTTP subset only; not public BFF/browser or provider proof.\n'
    }
    ;;
  --purchase-entry)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=120s -run '^TestMerchantPurchaseEntry' -v ./tests/foundation
  printf 'PASS: merchant purchase-entry real PG/HTTP subset only; not buyer UI or provider checkout.\n'
    }
    ;;
  --merchant-orders)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/merchant_orders_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestMerchantOrders' -v ./tests/foundation
  printf 'PASS: isolated merchant order read subset; no merchant UI/provider/deployment claim.\n'
    }
    ;;
  --inbox)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/live_console_inbox_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestLiveConsoleInbox' -v ./tests/foundation
  printf 'PASS: isolated live-console inbox read-side subset only; no UI/send (LC-B4) or LIVE Meta traffic claim.\n'
    }
    ;;
  --inbox-send)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/live_console_send_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=900s -run '^TestLiveConsoleSend' -v ./tests/foundation
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run 'Send|PrivateReplyFacts|Scrub|CheckText|Outbound|BodyHMAC|PublicReply' -v ./internal/inbox ./internal/httpapi ./internal/integrations/metareply
  printf 'PASS: isolated live-console sends + takeover (LCN06/07/08/10/11, display-copy half of LCN13) only; MOCK Graph, retention halves of LCN13 and LIVE sends (LCN16) not claimed.\n'
    }
    ;;
  --operations-queue)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/operations_queue_test.go
  # One process: the historical isolated-fixture gates cannot share a PG cluster with a second go test process (cluster-global roles). The dispatcher budget now
  # counts generation - generation_floor, so its budget/exhaustion/ambiguity gates and the function ACL pin must stay green beside the new gate.
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=900s -run '^(TestOperationsQueue|TestT06DispatcherBudgetPersistsManualRequirement|TestT06DispatcherFailedExhaustionCommitDoesNotCancelJob|TestT06DispatcherAmbiguityReconcilesWithoutRepeating|TestT06WorkerAuthorityAndFunctionACL|TestT06PolicyOutcomeCannotErasePossibleRemoteEffect)' -v ./tests/foundation
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run 'Ledger|Operation|Normalize' -v ./internal/integrations/core ./internal/httpapi ./internal/pagination ./internal/httperror
  printf 'PASS: isolated operations-ledger subset only (REAL_PG, MOCK provider): store isolation, exact DTO, cancel/query/retry rules, UNKNOWN-needs-proof with call-counting provider, concurrency, ACL pins; real Meta/ECPay/ads providers, the admin UI (W6-U2) and LIVE traffic are NOT_RUN.\n'
    }
    ;;
  --msg-templates)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/live_console_templates_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestLiveConsoleTemplates' -v ./tests/foundation
  printf 'PASS: isolated live-console message-templates subset only; no UI/send (LC-B4) or LIVE Meta traffic claim.\n'
    }
    ;;
  --meta-inbox)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/meta_inbox_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestMetaInbox' -v ./tests/foundation
  printf 'PASS: isolated Meta inbox subset only; no public mount/provider qualification claim.\n'
    }
    ;;
  --meta-consumer)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/meta_consumer_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestMetaConsumer' -v ./tests/foundation
  printf 'PASS: isolated Meta social consumer subset only; no public mount/provider qualification claim.\n'
    }
    ;;
  --meta-runtime)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/meta_runtime_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestMetaRuntime' -v ./tests/foundation
  printf 'PASS: isolated Meta API/worker runtime subset only; no public deployment/provider qualification claim.\n'
    }
    ;;
  --legacy-isolation)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/legacy_runtime_isolation_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=240s -run '^TestLegacyRuntimeIsolation' -v ./tests/foundation
  printf 'PASS: isolated legacy-family subset only; not full regression/provider/production acceptance.\n'
    }
    ;;
  --local-recovery)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  # This bounded gate creates its own source/restore clusters; the parent
  # fixture still enforces explicit local-PG consent. No existing DB is restored.
  test -f tests/foundation/local_recovery_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestLocalRecovery' -v ./tests/foundation
  printf 'PASS: isolated logical restore/cold-start subset only; not PITR, production RPO/RTO or deployment acceptance.\n'
    }
    ;;
  --live-planning)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/live_planning_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestLivePlanning' -v ./internal/live ./tests/foundation
  printf 'PASS: isolated live draft planning only; no broadcast, HTTP, provider or G06 acceptance.\n'
    }
    ;;
  --live-authority)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  # A selector with no matching test is not an authority gate.
  test -f tests/foundation/live_media_authorization_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^Test(LivePlanning|LiveMediaAuthorization)' -v ./tests/foundation
  printf 'PASS: isolated MOCK media authority registry and draft planning; no controller, LIVE intake, provider or G06 acceptance.\n'
    }
    ;;
  --live-media-plan)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  test -f tests/foundation/live_media_plan_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=240s -run '^Test(LivePlanning|LiveMediaAuthorization|LiveMediaPlan)' -v ./internal/live ./tests/foundation
  printf 'PASS: isolated MOCK media start intent and native queue; no provider execution, LIVE intake or G06 acceptance.\n'
    }
    ;;
  --live-media-execution)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  # Refuse a no-test success before provisioning any fixture.
  test -f tests/foundation/live_media_execution_test.go
  grep -q '^func TestLiveMediaExecution' tests/foundation/live_media_execution_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=300s -run '^Test(LivePlanning|LiveMediaAuthorization|LiveMediaPlan|LiveMediaExecution)' -v ./internal/live ./tests/foundation
  printf 'PASS: isolated MOCK media execution and recovery; no Stop, LIVE provider, resource reclamation or G06 acceptance.\n'
    }
    ;;
  --live-browser-input)
    lc_build=none
    lc_fixture=pg
    lc_covers="internal/integrations/livekit internal/live"
    lc_prepare() {
  test -f tests/foundation/live_browser_input_runtime_test.go
  grep -q '^func TestLiveBrowserInputBRW' tests/foundation/live_browser_input_runtime_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=240s -run '^TestLiveBrowserInputBRW' -v ./tests/foundation
  GOTOOLCHAIN=go1.27.2 go vet ./internal/live ./tests/foundation
  printf 'PASS: isolated BRW SQL/executor and Go HTTP subset; HTTPS browser/SFU and recovery gates remain separate.\n'
    }
    ;;
  --live-media-input)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  # A focused feedback loop, never a substitute for BIC05/full regression.
  test -f tests/foundation/live_media_input_custody_test.go
  grep -q '^func TestLiveMediaExecutionBIC' tests/foundation/live_media_input_custody_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestLiveMediaExecutionBIC' -v ./tests/foundation
  printf 'PASS: isolated BIC custody subset only; full media and BIC05 regression still required.\n'
    }
    ;;
  --live-media-crash)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  # Exact LMR05 diagnostic only, not acceptance of the Stop or full suite.
  grep -q '^func TestLiveMediaStopLMR05RealCrashAndCommitAckLoss(t \*testing.T)' tests/foundation/live_media_stop_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=600s -run '^TestLiveMediaStopLMR05RealCrashAndCommitAckLoss$' -v ./tests/foundation
  printf 'PASS: isolated LMR05 crash diagnostic only; Stop and full regression still required.\n'
    }
    ;;
  --live-media-stop)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  test -f tests/foundation/live_media_stop_test.go
  grep -q '^func TestLiveMediaStopLMR' tests/foundation/live_media_stop_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=300s -run '^Test(LivePlanning|LiveMediaAuthorization|LiveMediaPlan|LiveMediaExecution|LiveMediaStop)' -v ./internal/live ./tests/foundation
  printf 'PASS: isolated bounded MOCK media Stop; no Cloud, LIVE intake, operator escalation recovery or G06 acceptance.\n'
    }
    ;;
  --live-media-recovery)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  test -f tests/foundation/live_media_recovery_test.go
  grep -q '^func TestLiveMediaRecoveryMRR' tests/foundation/live_media_recovery_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -failfast -timeout=540s -run '^TestLiveMediaRecoveryMRR' -v ./tests/foundation
  printf 'PASS: isolated MRR observer SQL/process gates only; no Cloud, human alert delivery, LIVE intake or G06 acceptance.\n'
    }
    ;;
  --live-media-runtime)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  test -f tests/foundation/live_media_runtime_test.go
  test -f cmd/media-worker/main_test.go
  test -f internal/integrations/livekit/worker_env_test.go
  grep -q '^func TestLiveMediaRuntimeLMW' tests/foundation/live_media_runtime_test.go
  grep -q '^func TestMediaWorkerLMW' cmd/media-worker/main_test.go
  grep -q '^func TestWorkerEnvironmentLMW' internal/integrations/livekit/worker_env_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=360s -run '^Test(MediaWorkerLMW|WorkerEnvironmentLMW|LiveMediaRuntimeLMW)' -v ./cmd/media-worker ./internal/integrations/livekit ./tests/foundation
  printf 'PASS: isolated actual media command, PG18 and local TLS runtime; no Cloud, LIVE intake or G06 acceptance.\n'
    }
    ;;
  --live-console)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/foundation/live_console_comments_test.go
  GOTOOLCHAIN=go1.27.2 go test -race -count=1 -timeout=300s -run '^TestLiveConsoleLCN' -v ./internal/live ./tests/foundation
  printf 'PASS: isolated live-console comment read-through (LCN01/02/04/05, incl. IG comment-facts / facts_unavailable) only; MOCK Graph, no public mount or LIVE Meta acceptance.\n'
    }
    ;;
  --studio-backend)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  test -f tests/foundation/studio_backend_test.go
  test -f tests/foundation/studio_http_test.go
  test -f tests/foundation/studio_process_test.go
  test -f internal/pagination/studio_test.go
  grep -q '^func TestStudioBackend' tests/foundation/studio_backend_test.go
  grep -q '^func TestStudioBackendSTU03' tests/foundation/studio_process_test.go
  grep -q '^func TestStudioCursor' internal/pagination/studio_test.go
  test -f tests/foundation/studio_input_test.go
  grep -q '^func TestStudioInput' tests/foundation/studio_input_test.go
    }
    lc_run() {
  GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=360s -run '^Test(StudioBackend|StudioCursor|StudioInput)' -v ./internal/pagination ./internal/live ./internal/httpapi ./cmd/api ./tests/foundation
  printf 'PASS: isolated Studio backend/API and local MOCK media gate; not BFF/browser, Cloud, LIVE intake or full Studio acceptance.\n'
    }
    ;;
  --browser-design)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/buyer internal/buyerhttp internal/claims internal/design internal/mail internal/metaconnect internal/storefront internal/storefrontadmin internal/storefrontdomains internal/storehandles"
    lc_browsers=chromium
    lc_prepare() {
  # R3 storefront-publish KEY gate (BROWSER, MOCK edge): refuse a no-test success and run the pure card-model gate first.
  test -f tests/foundation/browser_storefront_publish_test.go
  grep -q '^func TestBrowserStorefrontPublish' tests/foundation/browser_storefront_publish_test.go
  test -f tests/storefront/storefront-publish-gate.mjs
  test -f tests/admin/design.spec.ts
  node --test --experimental-strip-types tests/admin/storefront-model.test.ts tests/admin/design-gate.test.ts tests/admin/design-model.test.ts packages/markdown-lite/tests/index.test.ts
  mkdir -p output/playwright
  test -f tests/foundation/browser_store_design_test.go
  grep -q '^func TestBrowserStoreDesign' tests/foundation/browser_store_design_test.go
    }
    lc_run() {
  node --test --experimental-strip-types tests/admin/design-model.test.ts tests/admin/design-gate.test.ts
  LC_BROWSER_STORE_DESIGN_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserStoreDesign$' -v ./tests/foundation
  printf 'PASS: store-design editor browser, signed MOCK IdP and isolated PG; not storefront rendering or production acceptance.\n'
    }
    ;;
  --browser-ops-polish)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp"
    lc_browsers=chromium
    lc_prepare() {
  # ops-polish independent gates (OP1 buyer half, OP2, OP3 UI half, OP4): refuse a no-test success before any build.
  test -f tests/foundation/browser_ops_polish_test.go
  grep -q '^func TestBrowserOpsPolishStorefront' tests/foundation/browser_ops_polish_test.go
  grep -q '^func TestBrowserOpsPolishAdmin' tests/foundation/browser_ops_polish_test.go
  test -f tests/admin/ops-polish.spec.ts
  test -f tests/storefront/cvs-pap-only.mjs
  # Pure model/copy contracts first (no Docker): finance row shapes, Studio subtitle, removed nav copy.
  node --test --experimental-strip-types tests/admin/ops-polish-model.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_OPS_POLISH_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run "^TestBrowserOpsPolish(${LC_OPS_POLISH_ONLY:-Storefront|Admin})\$" -v ./tests/foundation
  printf 'PASS: ops-polish isolated storefront + admin Next, Go, PG; OP1 buyer flow with no card payment service (zh-TW + en, desktop + 390px), OP2 order-feed polling on a fake clock, OP3 finance column/CSV link, OP4 Studio subtitle and nav in three locales; MOCK IdP, no PSP, no ECPay profile; not provider or deployment acceptance.\n'
    }
    ;;
  --browser-meta-connect)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/catalog internal/claims internal/csvguard internal/customers internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/meta/oauth internal/integrations/meta/pagetoken internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # meta-connect (BROWSER, Meta = MOCK fake Graph): refuse a no-test success and run the pure model/request gates first.
  test -f tests/foundation/browser_meta_connect_test.go
  grep -q '^func TestBrowserMetaConnect' tests/foundation/browser_meta_connect_test.go
  test -f tests/foundation/browser_meta_connect_gate_test.go
  grep -q '^func TestBrowserMetaConnectGate' tests/foundation/browser_meta_connect_gate_test.go
  test -f tests/admin/meta-connect.spec.ts
  test -f tests/admin/meta-connect-gate.spec.ts
  node --test --experimental-strip-types tests/admin/meta-connect-model.test.ts tests/admin/meta-connect-gate.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_META_CONNECT_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=2400s -run '^TestBrowserMetaConnect(Gate)?$' -v ./tests/foundation
  printf 'PASS: meta-connect isolated admin Next, Go API with the metaconnect service, PG; Meta = MOCK (tests/metaconnect/fakegraph + the Facebook Login dialog answered by the browser route); merchant connects Page A + Instagram, a forged state and a missing permission are refused, disconnect and a Facebook-only reconnect, en/zh-TW/zh-CN, desktop + 390px; the independent gate (MCG10) adds the Studio claim-source picker, state replay and hostile-cookie callback refusals; not Meta, provider or deployment acceptance; Meta SANDBOX/LIVE are NOT_RUN.\n'
    }
    ;;
  --browser-product-editor)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/checkout internal/domains internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # CC12 (BROWSER + REAL_PG): refuse a no-test success before any build; the admin Next build is the only web build this gate needs.
  test -f tests/foundation/browser_catalog_core_test.go
  grep -q '^func TestBrowserCatalogCore' tests/foundation/browser_catalog_core_test.go
  test -f tests/admin/catalog-core.spec.ts
  if [[ "$test_mode" == --browser-product-editor ]]; then
    test -f tests/admin/product-editor.acceptance.ts
    test -f tests/admin/product-review.acceptance.ts
  fi
  mkdir -p output/playwright
    }
    lc_run() {
  PRODUCT_EDITOR_ACCEPTANCE=1 LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserCatalogCore$' -v ./tests/foundation
  printf 'PASS: PE12-PE17 and review regressions on production admin Next, real Go/isolated PG, followed by mandatory frozen CC12; MOCK identity, no provider or deployment acceptance.\n'
    }
    ;;
  --browser-catalog-core)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/checkout internal/domains internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/mail internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # CC12 (BROWSER + REAL_PG): refuse a no-test success before any build; the admin Next build is the only web build this gate needs.
  test -f tests/foundation/browser_catalog_core_test.go
  grep -q '^func TestBrowserCatalogCore' tests/foundation/browser_catalog_core_test.go
  test -f tests/admin/catalog-core.spec.ts
  if [[ "$test_mode" == --browser-product-editor ]]; then
    test -f tests/admin/product-editor.acceptance.ts
    test -f tests/admin/product-review.acceptance.ts
  fi
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_CATALOG_CORE_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserCatalogCore$' -v ./tests/foundation
  printf 'PASS: CC12 isolated admin Next, Go API, PG; shopper view = the real buyer catalog v2 HTTP handler (not the storefront Next build); desktop + 390 px, en + zh-TW; not provider or deployment acceptance.\n'
    }
    ;;
  --ops-disk-guard)
    lc_build=none
    lc_fixture=none
    lc_prepare() {
  :
    }
    lc_run() {
  test -f tests/deploy/ops-disk-guard.sh
  mkdir -p "$LC_OD_EVIDENCE"
  bash tests/deploy/ops-disk-guard.sh all 2>&1 | tee "$LC_OD_EVIDENCE/od-all.log"
  exit 0
    }
    ;;
  --browser-returns-ui)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/checkout internal/claims internal/csvguard internal/domains internal/fulfillment internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/jobqueue internal/merchantorders internal/promotions internal/returns internal/storefront"
    lc_browsers=chromium
    lc_prepare() {
  # W3-U5 (BROWSER): refuse a no-test success and run the pure returns model/BFF-grammar gate first.
  test -f tests/foundation/browser_returns_ui_test.go
  grep -q '^func TestBrowserReturnsUI' tests/foundation/browser_returns_ui_test.go
  test -f tests/admin/returns-ui.spec.ts
  node --test --experimental-strip-types tests/admin/returns-model.test.ts tests/admin/returns-request.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_RETURNS_UI_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run '^TestBrowserReturnsUI$' -v ./tests/foundation
  printf 'PASS: W3-U5 returns/cancel UI: isolated admin Next, Go, PG; register/receive/inspect/close/withdraw RMA, merchant cancel refusals (refund_first, payment_in_flight), cancel-refund-gaps list, permission gating; zh-TW + en, desktop + 390px; MOCK providers only — not provider or deployment acceptance.\n'
    }
    ;;
  --browser-picklist)
    lc_build=none
    lc_fixture=none
    lc_covers=""
    lc_browsers=chromium
    lc_prepare() {
  :
    }
    lc_run() {
  source scripts/dev/test-lock.sh
  lc_lock_acquire "${LC_TEST_LOCK_WAIT:-300}" || exit 2
  trap lc_lock_release EXIT
  mkdir -p "$LC_BROWSER_EVIDENCE_ROOT/picklist"
  pnpm --filter @live-commerce/admin build > "$LC_BROWSER_EVIDENCE_ROOT/picklist/build.log" 2>&1
  node --test --test-reporter=spec --experimental-strip-types tests/admin/picklist.spec.ts
  exit 0
    }
    ;;
  --browser-operations-ads)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/claims"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_operations_ads_test.go
  grep -q "^func TestBrowserOperationsAds" tests/foundation/browser_operations_ads_test.go
  test -f tests/admin/operations-ads.spec.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_OPERATIONS_ADS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1500s -run "^TestBrowserOperationsAds$" -v ./tests/foundation
  printf "PASS: W6-U2 BROWSER + REAL_PG + MOCK; no SANDBOX/LIVE or deployment acceptance.\n"
    }
    ;;
  --browser-product-media-v2)
    lc_build=both
    lc_fixture=pg
    lc_covers="internal/buyer internal/buyerhttp internal/catalog internal/inventory internal/mail"
    lc_browsers=chromium
    lc_prepare() {
  test -f tests/foundation/browser_product_media_v2_test.go
  test -f tests/foundation/browser_product_media_v2_fixture_test.go
  grep -q '^func TestBrowserProductMediaV2RealUpload' tests/foundation/browser_product_media_v2_test.go
  test -f tests/admin/product-media-v2.spec.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_BROWSER_PRODUCT_MEDIA_V2_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=1700s -run '^TestBrowserProductMediaV2RealUpload$' -v ./tests/foundation
  printf 'PASS: PM-U real uploads and buyer variant/detail/cart/checkout on production admin + storefront Next, Go and isolated PG; MOCK identity/edge, no provider or deployment acceptance.\n'
    }
    ;;
  --browser-reports)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/billing internal/catalog internal/claims internal/csvguard internal/customers internal/design internal/fulfillment internal/inbox internal/integrations/accounts internal/integrations/core internal/integrations/psp/payuni internal/integrations/shipping/ecpay internal/inventory internal/jobqueue internal/live internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/msgtemplates internal/notify internal/payments internal/payments/platformstripe internal/payments/settlement internal/pricing internal/promotions internal/reporting internal/returns internal/storefrontadmin internal/storefrontdomains internal/storehandles internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # W6-U1: four actual reports and audited CSVs; signed HTTPS and real scoped Go/PG fixtures.
  test -f tests/foundation/browser_w6_customers_reports_test.go
  grep -q '^func TestBrowserW6Reports' tests/foundation/browser_w6_customers_reports_test.go
  test -f tests/admin/reports.spec.ts
  node --test --experimental-strip-types tests/admin/reports-bff.test.ts tests/admin/reports-render.test.mjs
  mkdir -p output/playwright
    }
    lc_run() {
  LC_W6UI_REPORTS_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=2400s -run '^TestBrowserW6Reports$' -v ./tests/foundation
  printf 'PASS: W6-U1 four reports, 92-day range, sorted grouped amounts, LIVE/SANDBOX and offline splits, audited CSV permissions and fences; signed MOCK IdP, real Go/PG. Not provider or production acceptance.\n'

    }
    ;;
  --browser-migration-import)
    lc_build=admin
    lc_fixture=pg
    lc_covers="internal/ads internal/claims internal/csvguard internal/customers internal/fulfillment internal/integrations/shipping/ecpay internal/mail internal/merchantorders internal/merchanttools internal/metaconnect internal/migrationimport internal/promotions internal/reporting internal/returns internal/twcity"
    lc_browsers=chromium
    lc_prepare() {
  # W5-U1: actual signed HTTPS UI and scoped Go/PG, original CSV bytes and no invented status endpoint.
  test -f tests/foundation/browser_migration_import_ui_test.go
  grep -q '^func TestBrowserMigrationImport' tests/foundation/browser_migration_import_ui_test.go
  test -f tests/admin/import-wizard.spec.ts
  node --test --experimental-strip-types tests/admin/import-wire-model.test.ts tests/admin/import-wire-header.test.ts tests/admin/import-client.test.mjs tests/admin/import-history-model.test.ts tests/admin/import-coordinator.test.mjs tests/admin/import-view.test.ts tests/admin/import-integration.test.ts
  mkdir -p output/playwright
    }
    lc_run() {
  LC_MIUI_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.2 go test -race -tags browser -count=1 -timeout=2400s -run '^TestBrowserMigrationImport$' -v ./tests/foundation
  printf 'PASS: W5-U1 genuine customer/history CSV clicks, immutable file/mapping/count replay, safe row-only preview/download, UNKNOWN and stale recovery, scoped consent/erasure/city counters and readonly archive; real Go/PG with signed MOCK IdP. Not provider or production acceptance.\n'

    }
    ;;
  foundation)
    lc_build=none
    lc_fixture=pg
    lc_prepare() {
  :
    }
    lc_run() {
  # Serial gates include two independent worker crash-rescue suites and fresh
  # migration clusters. Only this additive package envelope grows; individual
  # gate deadlines and production rescue defaults are unchanged.
  # Run package binaries serially: independent author and root parallel runs
  # stalled before checkout's test output on this host. This does not disable
  # -race, in-test concurrency or any test; it is not a product root-cause fix.
  # Isolated Meta cutover/maintenance gates add fresh clusters and real process
  # windows. The aggregate exceeded 360s without an assertion failure; retain
  # every individual SQL/process deadline and allow the complete suite to finish.
  # Root measured 483.549s before LMR; the Stop-inclusive focused suite took
  # 180.126s, adding roughly 113s to the full package. This 900s envelope
  # covers aggregate tests only; no SQL, lease, 5s pacing or fault gate changes.
  # Serial real-clock MRR cases add ~344s to the measured ~715s baseline.
  # This suite envelope does not change recovery's 90s gate or the separately
  # owner-approved LMR05 90s wait; their safety predicates remain unchanged.
  # 2026-09-29: the GitHub runner took ~1449s for this package on bef13f2 (before Stripe B1);
  # adding the SP06-SP21 gates pushed daf08ee past 1500s (panic: test timed out after 25m0s,
  # while TestStripeSP10Deadline was 22s in). 2700s keeps headroom inside the 60 min CI job.
  # 2026-09-30: with the R2 lanes merged the foundation package alone needs ~54 min on the dev Mac (release gate at
  # 57c5aaa: panic "test timed out after 45m0s" with 61 tests not started; those took a further 527 s). -timeout is a
  # hang bound, not a gate: 4500s, and the CI job bound moves to 90 min with it (.github/workflows/foundation.yml).
  # CI shards (2026-10-06, the serial suite reached 76 min on GitHub): LC_FOUNDATION_PKGS narrows the package set,
  # LC_FOUNDATION_RUN the -run regex and LC_FOUNDATION_SKIP the -skip regex; .github/workflows/gates.yml runs 1 unit shard + the
  # tests/foundation groups of scripts/dev/shard-plan.json in parallel (each top-level test in exactly one group, enforced by
  # `node scripts/dev/shard-plan.mjs --check` in check-gates.sh). Unset = the whole suite, unchanged.
  # shellcheck disable=SC2086 # LC_FOUNDATION_PKGS is a deliberate word-split package list
  GOTOOLCHAIN=go1.27.2 go test -p 1 -race -count=1 -timeout=4500s -v ${LC_FOUNDATION_RUN:+-run "$LC_FOUNDATION_RUN"} ${LC_FOUNDATION_SKIP:+-skip "$LC_FOUNDATION_SKIP"} ${LC_FOUNDATION_PKGS:-./...}
  # go vet ./... checks the whole repository, not the shard's tests: a tests/foundation test-subset shard (RUN/SKIP set) leaves it to the
  # unit shard / the whole-suite run, so a full run vets exactly once instead of once per shard.
  if [[ -z "${LC_FOUNDATION_RUN:-}${LC_FOUNDATION_SKIP:-}" ]]; then GOTOOLCHAIN=go1.27.2 go vet ./...; fi
  printf 'PASS: isolated real PostgreSQL foundation tests; fixture removed at exit.\n'
    }
    ;;
  # APPEND MODES HERE
  *) return 1 ;;
esac
}
# END MODE REGISTRY

# Discovery only reads this registry; no prerequisites, build, fixture or mode command executes.
lc_mode_names() {
  local line declaration mode active=0 function_indent="" seen='|'
  local valid='^[[:blank:]]*(--[a-z0-9-]+|foundation)\)[[:blank:]]*$'
  local function_start='^([[:blank:]]+)lc_(prepare|run)\(\) \{'
  while IFS= read -r line; do
    if [[ "$line" == '# BEGIN MODE REGISTRY' ]]; then active=1; continue; fi
    if [[ "$line" == '# END MODE REGISTRY' ]]; then
      if [[ -n "$function_indent" ]]; then printf 'unclosed registry command function\n' >&2; return 2; fi
      break
    fi
    [[ "$active" == 0 ]] && continue
    if [[ -n "$function_indent" ]]; then
      [[ "$line" == "$function_indent}" ]] && function_indent=""
      continue
    fi
    if [[ "$line" =~ $function_start ]]; then
      [[ "$line" != *'}' ]] && function_indent="${BASH_REMATCH[1]}"
      continue
    fi
    declaration="${line#"${line%%[![:space:]]*}"}"
    declaration="${declaration%"${declaration##*[![:space:]]}"}"
    case "$declaration" in
      ''|'#'*|'lc_select_mode() {'|'case "$1" in'|'fi'|';;'|'esac'|'}'|'*) return 1 ;;') continue ;;
      lc_build=*|lc_fixture=*|lc_browsers=chromium|'lc_browsers="chromium webkit"'|lc_covers=*|'if [['*']]; then') continue ;;
    esac
    if [[ ! "$line" =~ $valid ]]; then printf 'invalid mode registry declaration\n' >&2; return 2; fi
    mode="${BASH_REMATCH[1]}"
    if [[ "$seen" == *"|$mode|"* ]]; then printf 'duplicate mode registry arm\n' >&2; return 2; fi
    seen="$seen$mode|"; printf '%s\n' "$mode"
  done < scripts/dev/test-local.sh
}

lc_usage() {
  local mode choices="" separator=""
  while IFS= read -r mode; do
    [[ "$mode" == foundation ]] && continue
    choices="$choices$separator$mode"; separator='|'
  done < <(lc_mode_names)
  printf 'Usage: bash scripts/dev/test-local.sh [%s]\n' "$choices"
}
lc_build_storefront() {
  command -v pnpm >/dev/null
  command -v openssl >/dev/null
  COMMERCE_BUYER_WEB_ENABLED=0 pnpm run build:storefront
  mkdir -p output/playwright
}
lc_build_admin() {
  command -v pnpm >/dev/null
  command -v node >/dev/null
  # Production package, but local-only runtime configuration is injected by the
  # tagged test. Build never needs an IdP or database credential.
  COMMERCE_IDENTITY_ENABLED=0 COMMERCE_FIXTURE_ENABLED=0 pnpm run build:admin
}

lc_environment() {
  export LC_PLATFORM_HOST=platform.example.invalid LC_ADMIN_HOST=admin.example.invalid LC_COMPANY_CONTACT_EMAIL=contact@example.invalid
}

if [[ "$#" == 1 && "$1" == --list ]]; then lc_mode_names; exit 0; fi
lc_dry_run=0
if [[ "${1:-}" == --dry-run ]]; then lc_dry_run=1; shift; fi
if [[ "$lc_dry_run" == 0 ]]; then
  command -v docker >/dev/null
  command -v go >/dev/null
  command -v node >/dev/null
  # Synthetic build configuration, never production defaults.
  lc_environment
fi
if [[ "$#" -gt 1 ]]; then lc_usage >&2; exit 2; fi
test_mode="${1:-foundation}"
if [[ "$lc_dry_run" == 0 ]]; then
# Allocate once before any browser build/log write. Explicit caller roots are kept,
# made absolute and shared by shell, Go and Node; the default is always ignored.
case "$test_mode" in
  --browser*|--stripe-browser*|--ops-disk-guard)
    if [[ -z "${LC_BROWSER_EVIDENCE_ROOT:-}" ]]; then
      mkdir -p output/playwright
      LC_BROWSER_EVIDENCE_ROOT="$(mktemp -d "$PWD/output/playwright/run.XXXXXXXX")"
    else
      lc_root_existed=0; [[ -d "$LC_BROWSER_EVIDENCE_ROOT" ]] && lc_root_existed=1
      mkdir -p "$LC_BROWSER_EVIDENCE_ROOT"
      LC_BROWSER_EVIDENCE_ROOT="$(cd "$LC_BROWSER_EVIDENCE_ROOT" && pwd -P)"
      # Fixed children (ui-click-sweep, platform-site, …) would overwrite committed evidence under a tracked root such as
      # output/: inside the repo, accept only an ignored directory with no tracked files (PR #23 review).
      lc_repo_top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
      [[ -n "$lc_repo_top" ]] && lc_repo_top="$(cd "$lc_repo_top" && pwd -P)"   # physical on both sides: symlinked paths cannot bypass
      case "$LC_BROWSER_EVIDENCE_ROOT/" in
        "${lc_repo_top:-/nonexistent-repo}"/*)
          if [[ -n "$(git ls-files -- "$LC_BROWSER_EVIDENCE_ROOT" | head -1)" ]] || ! git check-ignore -q "$LC_BROWSER_EVIDENCE_ROOT/.lc-evidence-probe"; then
            echo "test-local: LC_BROWSER_EVIDENCE_ROOT=$LC_BROWSER_EVIDENCE_ROOT is inside the repository but is not an ignored, untracked directory (use output/playwright/…)" >&2
            (( lc_root_existed )) || rmdir "$LC_BROWSER_EVIDENCE_ROOT" 2>/dev/null || true   # leave no refused root behind
            exit 2
          fi ;;
      esac
      # A reused root must not redirect a fixed child (ui-click-sweep, platform-site, …) through a symlink (PR #23 review).
      for lc_child in "$LC_BROWSER_EVIDENCE_ROOT"/*; do
        [[ -L "$lc_child" ]] && { echo "test-local: $lc_child is a symlink inside LC_BROWSER_EVIDENCE_ROOT; refusing" >&2; exit 2; }
      done
    fi
    export LC_BROWSER_EVIDENCE_ROOT
    if [[ "$test_mode" == --ops-disk-guard ]]; then
      export LC_OD_EVIDENCE="${LC_OD_EVIDENCE:-$LC_BROWSER_EVIDENCE_ROOT/ops-disk-guard}"
    fi
    export LC_PLATFORM_EVIDENCE="$LC_BROWSER_EVIDENCE_ROOT/platform-site"
    export LC_SWEEP_OUT="$LC_BROWSER_EVIDENCE_ROOT/ui-click-sweep"
    # A caller may reuse its run root: visual empty fixtures must not overwrite click evidence.
    if [[ "$test_mode" == --browser-visual-lint ]]; then
      export LC_SWEEP_OUT="$LC_BROWSER_EVIDENCE_ROOT/ui-visual-sweep"
    fi
    ;;
esac

fi
if ! lc_select_mode "$test_mode"; then lc_usage >&2; exit 2; fi
if [[ "$lc_dry_run" == 1 ]]; then
  printf '# mode=%s build=%s fixture=%s\n' "$test_mode" "$lc_build" "$lc_fixture"
  declare -f lc_environment
  printf 'lc_environment\n'
  declare -f lc_prepare
  printf 'lc_prepare\n'
  if [[ "$lc_build" == storefront || "$lc_build" == both ]]; then declare -f lc_build_storefront; printf 'lc_build_storefront\n'; fi
  if [[ "$lc_build" == admin || "$lc_build" == both ]]; then declare -f lc_build_admin; printf 'lc_build_admin\n'; fi
  if [[ "$lc_fixture" == pg ]]; then cat scripts/dev/test-local-runtime.sh; fi
  declare -f lc_run
  printf 'lc_run\n'
  exit 0
fi
lc_prepare
if [[ "$lc_build" == storefront || "$lc_build" == both ]]; then lc_build_storefront; fi
if [[ "$lc_build" == admin || "$lc_build" == both ]]; then lc_build_admin; fi
if [[ "$lc_fixture" == pg ]]; then source scripts/dev/test-local-runtime.sh; fi
lc_run
