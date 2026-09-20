#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
command -v docker >/dev/null
command -v go >/dev/null
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
for ((attempt=0; attempt<40; attempt++)); do
  if docker exec "$test_container" pg_isready -U postgres -d lc_foundation_test >/dev/null 2>&1; then break; fi
  sleep 0.5
done
docker exec "$test_container" pg_isready -U postgres -d lc_foundation_test >/dev/null
test_port="$(docker port "$test_container" 5432/tcp)"
[[ "$test_port" == 127.0.0.1:* ]]
export LC_TEST_DATABASE_URL="postgres://postgres:${POSTGRES_PASSWORD}@${test_port}/lc_foundation_test?sslmode=disable"
export LC_TEST_DATABASE_ALLOWED=1
GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -v ./...
GOTOOLCHAIN=go1.27.1 go vet ./...
printf 'PASS: isolated real PostgreSQL foundation tests; fixture removed at exit.\n'
