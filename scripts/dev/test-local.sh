#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
command -v docker >/dev/null
command -v go >/dev/null
# PAYUNi protocol tests compare Go forms with independent Node/OpenSSL and the
# public official golden vector; missing Node must fail before starting fixtures.
command -v node >/dev/null
test_mode="${1:-foundation}"
if [[ "$#" -gt 1 ]] || [[ "$test_mode" != foundation && "$test_mode" != --browser-identity && "$test_mode" != --checkout && "$test_mode" != --payment ]]; then
  printf 'Usage: bash scripts/dev/test-local.sh [--browser-identity|--checkout|--payment]\n' >&2
  exit 2
fi
if [[ "$test_mode" == --browser-identity ]]; then
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
elif [[ "$test_mode" == --checkout ]]; then
  # Focused diagnosis uses the same isolated real PG and cleanup guard. It never
  # substitutes for the full foundation/race/vet release gate below.
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run '^TestBuyerCheckout' -v ./tests/foundation
  printf 'PASS: checkout subset only; full regression still required.\n'
elif [[ "$test_mode" == --payment ]]; then
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run '^TestBuyerPayment' -v ./tests/foundation
  printf 'PASS: payment start/query subset only; full regression still required.\n'
else
  # The growing serial real-PG suite includes a deliberate ~35s process-crash
  # rescue. This is the package envelope, not a relaxation of per-case fences.
  GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=240s -v ./...
  GOTOOLCHAIN=go1.27.1 go vet ./...
  printf 'PASS: isolated real PostgreSQL foundation tests; fixture removed at exit.\n'
fi
