#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
command -v docker >/dev/null
command -v go >/dev/null
# PAYUNi protocol tests compare Go forms with independent Node/OpenSSL and the
# public official golden vector; missing Node must fail before starting fixtures.
command -v node >/dev/null
test_mode="${1:-foundation}"
if [[ "$#" -gt 1 ]] || [[ "$test_mode" != foundation && "$test_mode" != --browser-identity && "$test_mode" != --browser-buyer && "$test_mode" != --browser-merchant-buyer && "$test_mode" != --browser-merchant-orders-bff && "$test_mode" != --browser-order && "$test_mode" != --browser-payment && "$test_mode" != --checkout && "$test_mode" != --payment && "$test_mode" != --payment-worker && "$test_mode" != --expiry-worker && "$test_mode" != --storefront-resolver && "$test_mode" != --buyer-http && "$test_mode" != --purchase-entry && "$test_mode" != --merchant-orders && "$test_mode" != --meta-inbox && "$test_mode" != --meta-consumer && "$test_mode" != --meta-runtime ]]; then
  printf 'Usage: bash scripts/dev/test-local.sh [--browser-identity|--browser-buyer|--browser-merchant-buyer|--browser-merchant-orders-bff|--browser-order|--browser-payment|--checkout|--payment|--payment-worker|--expiry-worker|--storefront-resolver|--buyer-http|--purchase-entry|--merchant-orders|--meta-inbox|--meta-consumer|--meta-runtime]\n' >&2
  exit 2
fi
if [[ "$test_mode" == --browser-merchant-buyer ]]; then
  # Do not report a pass from an exact Go test selector matching no test.
  test -f tests/foundation/browser_merchant_buyer_chain_test.go
fi
if [[ "$test_mode" == --browser-order ]]; then
  test -f tests/foundation/browser_order_chain_test.go
fi
if [[ "$test_mode" == --browser-payment ]]; then
  test -f tests/foundation/browser_payment_chain_test.go
fi
if [[ "$test_mode" == --browser-merchant-orders-bff ]]; then
  test -f tests/foundation/browser_merchant_orders_bff_test.go
  test -f tests/admin/orders-bff.spec.ts
  # Raw URL grammar is shared by Proxy and route; real HTTP below additionally
  # proves that the framework cannot normalize a rejected request past it.
  node --test --experimental-strip-types tests/admin/orders-request.test.ts
  mkdir -p output/playwright
fi
if [[ "$test_mode" == --browser-buyer || "$test_mode" == --browser-merchant-buyer || "$test_mode" == --browser-order || "$test_mode" == --browser-payment ]]; then
  command -v pnpm >/dev/null
  command -v openssl >/dev/null
  COMMERCE_BUYER_WEB_ENABLED=0 pnpm run build:storefront
  mkdir -p output/playwright
fi
if [[ "$test_mode" == --browser-identity || "$test_mode" == --browser-merchant-buyer || "$test_mode" == --browser-merchant-orders-bff ]]; then
  command -v pnpm >/dev/null
  command -v node >/dev/null
  # Production package, but local-only runtime configuration is injected by the
  # tagged test. Build never needs an IdP or database credential.
  COMMERCE_IDENTITY_ENABLED=0 COMMERCE_FIXTURE_ENABLED=0 pnpm run build:admin
fi
# A task-owned, temporary PG only. Never use a developer's existing DATABASE_URL.
test_container="lc-foundation-test-$$"
test_owned=0
cleanup() {
  if [[ "$test_owned" == 1 ]] && [[ "$(docker inspect -f '{{index .Config.Labels "livecommerce.fixture"}}' "$test_container" 2>/dev/null || true)" == "$test_container" ]]; then
    docker rm -f "$test_container" >/dev/null
  fi
}
trap cleanup EXIT INT TERM
export POSTGRES_PASSWORD
POSTGRES_PASSWORD="$(openssl rand -hex 24)"
docker run -d --pull=never --name "$test_container" \
  --label "livecommerce.fixture=$test_container" --memory=512m --cpus=1 --pids-limit=128 \
  --tmpfs /var/lib/postgresql:rw,size=268435456 \
  -e POSTGRES_PASSWORD -e POSTGRES_DB=lc_foundation_test \
  -p 127.0.0.1::5432 \
  postgres@sha256:4ef4dbc939d61acea57712655ddb4b4ab27419c913f94cca0cd57cb3ea3c2280 \
  -c shared_buffers=32MB -c max_connections=30 >/dev/null
test_owned=1
# The image starts a socket-only temporary server during initdb, then stops it.
# TCP readiness must wait for the final server; socket pg_isready can race createdb.
for ((attempt=0; attempt<40; attempt++)); do
  if docker exec "$test_container" pg_isready -h 127.0.0.1 -U postgres -d lc_foundation_test >/dev/null 2>&1; then break; fi
  sleep 0.5
done
docker exec "$test_container" pg_isready -h 127.0.0.1 -U postgres -d lc_foundation_test >/dev/null
# The fixture safety guard needs a distinct, explicitly disposable database.
# Reuse this owned cluster, not a developer's running UI fixture or credentials.
docker exec "$test_container" createdb -U postgres lc_admin_fixture
test_port="$(docker port "$test_container" 5432/tcp)"
[[ "$test_port" == 127.0.0.1:* ]]
export LC_TEST_DATABASE_URL="postgres://postgres:${POSTGRES_PASSWORD}@${test_port}/lc_foundation_test?sslmode=disable"
export LC_TEST_DATABASE_ALLOWED=1
export COMMERCE_FIXTURE_ALLOWED=1
export LC_ADMIN_GUARD_DSN="postgres://postgres:${POSTGRES_PASSWORD}@${test_port}/lc_admin_fixture?sslmode=disable"
if [[ "$test_mode" == --browser-identity ]]; then
  LC_BROWSER_IDENTITY_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.1 go test -race -tags browser -count=1 -timeout=180s -run '^(TestBrowserIdentityRealChain|TestBrowserSettingsWizardRealChain|TestMerchantAccountAPIProcessRestart)$' -v ./tests/foundation
  printf 'PASS: isolated PG + signed MOCK IdP browser chain; fixture removed at exit.\n'
elif [[ "$test_mode" == --browser-merchant-orders-bff ]]; then
  LC_BROWSER_MERCHANT_ORDERS_BFF_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.1 go test -race -tags browser -count=1 -timeout=180s -run '^TestBrowserMerchantOrdersBFFRealChain$' -v ./tests/foundation
  printf 'PASS: isolated Next + Go + PG merchant-order read transport; not merchant UI or provider acceptance.\n'
elif [[ "$test_mode" == --browser-buyer ]]; then
  LC_BROWSER_BUYER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.1 go test -race -tags browser -count=1 -timeout=180s -run '^TestBrowserBuyerRealChain$' -v ./tests/foundation
  printf 'PASS: isolated PG + real buyer browser transport; not UI/PSP/deployment acceptance.\n'
elif [[ "$test_mode" == --browser-merchant-buyer ]]; then
  LC_BROWSER_MERCHANT_BUYER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.1 go test -race -tags browser -count=1 -timeout=180s -run '^TestBrowserMerchantBuyerRealChain$' -v ./tests/foundation
  printf 'PASS: isolated merchant-to-buyer browser chain; not provider payment or real DNS/TLS deployment proof.\n'
elif [[ "$test_mode" == --browser-order ]]; then
  LC_BROWSER_ORDER_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.1 go test -race -tags browser -count=1 -timeout=180s -run '^TestBrowserBuyerOrderUI$' -v ./tests/foundation
  printf 'PASS: isolated buyer address/order UI gate; not provider payment or deployment acceptance.\n'
elif [[ "$test_mode" == --browser-payment ]]; then
  LC_BROWSER_PAYMENT_ACCEPTANCE=1 GOTOOLCHAIN=go1.27.1 go test -race -tags browser -count=1 -timeout=180s -run '^TestBrowserBuyerPaymentUI$' -v ./tests/foundation
  printf 'PASS: isolated buyer payment UI/native POST gate with a local mock PSP; not provider payment or deployment acceptance.\n'
elif [[ "$test_mode" == --checkout ]]; then
  # Focused diagnosis uses the same isolated real PG and cleanup guard. It never
  # substitutes for the full foundation/race/vet release gate below.
  # This selector now includes the expiry-worker crash/rescue suites as well as
  # checkout. Their aggregate exceeded 120s; individual SQL/deadline gates stay.
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=240s -run '^TestBuyerCheckout' -v ./tests/foundation
  printf 'PASS: checkout subset only; full regression still required.\n'
elif [[ "$test_mode" == --payment ]]; then
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=180s -run '^TestBuyerPayment' -v ./tests/foundation
  printf 'PASS: payment start/query subset only; full regression still required.\n'
elif [[ "$test_mode" == --payment-worker ]]; then
  test -f tests/foundation/payment_runtime_test.go
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run '^TestBuyerPaymentWorker' -v ./tests/foundation
  printf 'PASS: isolated payment worker subset; no real-provider or deployment claim.\n'
elif [[ "$test_mode" == --expiry-worker ]]; then
  test -f tests/foundation/expiry_runtime_test.go
  test -f tests/foundation/expiry_admission_test.go
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=150s -run '^TestBuyerCheckoutExpiryRuntime' -v ./tests/foundation
  printf 'PASS: isolated expiry worker subset; no production or recovery-SLO claim.\n'
elif [[ "$test_mode" == --merchant-orders ]]; then
  test -f tests/foundation/merchant_orders_test.go
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=180s -run '^TestMerchantOrders' -v ./tests/foundation
  printf 'PASS: isolated merchant order read subset; no merchant UI/provider/deployment claim.\n'
elif [[ "$test_mode" == --meta-inbox ]]; then
  test -f tests/foundation/meta_inbox_test.go
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=180s -run '^TestMetaInbox' -v ./tests/foundation
  printf 'PASS: isolated Meta inbox subset only; no public mount/provider qualification claim.\n'
elif [[ "$test_mode" == --meta-consumer ]]; then
  test -f tests/foundation/meta_consumer_test.go
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=180s -run '^TestMetaConsumer' -v ./tests/foundation
  printf 'PASS: isolated Meta social consumer subset only; no public mount/provider qualification claim.\n'
elif [[ "$test_mode" == --meta-runtime ]]; then
  test -f tests/foundation/meta_runtime_test.go
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=180s -run '^TestMetaRuntime' -v ./tests/foundation
  printf 'PASS: isolated Meta API/worker runtime subset only; no public deployment/provider qualification claim.\n'
elif [[ "$test_mode" == --storefront-resolver ]]; then
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run '^TestPublishedStorefront' -v ./tests/foundation
  printf 'PASS: published-origin resolver subset only; not public HTTP or provider proof.\n'
elif [[ "$test_mode" == --buyer-http ]]; then
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run '^TestBuyerHTTP' -v ./tests/foundation
  printf 'PASS: private buyer HTTP subset only; not public BFF/browser or provider proof.\n'
elif [[ "$test_mode" == --purchase-entry ]]; then
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run '^TestMerchantPurchaseEntry' -v ./tests/foundation
  printf 'PASS: merchant purchase-entry real PG/HTTP subset only; not buyer UI or provider checkout.\n'
else
  # Serial gates include two independent worker crash-rescue suites and fresh
  # migration clusters. Only this additive package envelope grows; individual
  # gate deadlines and production rescue defaults are unchanged.
  # Run package binaries serially: independent author and root parallel runs
  # stalled before checkout's test output on this host. This does not disable
  # -race, in-test concurrency or any test; it is not a product root-cause fix.
  # Isolated Meta cutover/maintenance gates add fresh clusters and real process
  # windows. The aggregate exceeded 360s without an assertion failure; retain
  # every individual SQL/process deadline and allow the complete suite to finish.
  GOTOOLCHAIN=go1.27.1 go test -p 1 -race -count=1 -timeout=600s -v ./...
  GOTOOLCHAIN=go1.27.1 go vet ./...
  printf 'PASS: isolated real PostgreSQL foundation tests; fixture removed at exit.\n'
fi
