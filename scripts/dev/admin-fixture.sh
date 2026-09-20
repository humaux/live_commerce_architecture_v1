#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
command -v docker >/dev/null
command -v go >/dev/null
name="lc-admin-fixture-$$"; owned=0; api_pid=""
cleanup(){
  [[ -n "$api_pid" ]] && kill "$api_pid" 2>/dev/null || true
  if [[ "$owned" == 1 ]] && [[ "$(docker inspect -f '{{index .Config.Labels "livecommerce.fixture"}}' "$name" 2>/dev/null || true)" == "$name" ]]; then docker rm -f "$name" >/dev/null; fi
}
trap cleanup ERR INT TERM
password="$(openssl rand -hex 24)"
docker run -d --pull=never --name "$name" --label "livecommerce.fixture=$name" --memory=512m --cpus=1 --pids-limit=128 --tmpfs /var/lib/postgresql:rw,size=268435456 -e POSTGRES_PASSWORD="$password" -e POSTGRES_DB=lc_admin_fixture -p 127.0.0.1::5432 postgres@sha256:4ef4dbc939d61acea57712655ddb4b4ab27419c913f94cca0cd57cb3ea3c2280 -c shared_buffers=32MB -c max_connections=30 >/dev/null
owned=1
for ((i=0;i<40;i++)); do docker exec "$name" pg_isready -U postgres -d lc_admin_fixture >/dev/null 2>&1 && break; sleep .5; done
docker exec "$name" pg_isready -U postgres -d lc_admin_fixture >/dev/null
host="$(docker port "$name" 5432/tcp)"; [[ "$host" == 127.0.0.1:* ]]
bin="$(mktemp -d)/admin-fixture"
GOTOOLCHAIN=go1.27.1 go build -o "$bin" ./cmd/admin-fixture
seed="$(FIXTURE_OWNER_DSN="postgres://postgres:${password}@${host}/lc_admin_fixture?sslmode=disable" FIXTURE_HOST="$host" FIXTURE_DB=lc_admin_fixture "$bin")"
store="$(printf '%s\n' "$seed" | sed -n 's/^STORE_ID=//p')"; token="$(printf '%s\n' "$seed" | sed -n 's/^TOKEN=//p')"; api_password="$(printf '%s\n' "$seed" | sed -n 's/^API_PASSWORD=//p')"
[[ "$store" =~ ^[0-9a-f-]{36}$ && ${#token} -ge 32 ]]
envfile="$(mktemp /tmp/livecommerce-admin-fixture.XXXXXX)"; chmod 600 "$envfile"
printf 'COMMERCE_FIXTURE_ENABLED=1\nCOMMERCE_API_ORIGIN=http://127.0.0.1:18081\nCOMMERCE_FIXTURE_STORE_ID=%s\nCOMMERCE_FIXTURE_TOKEN=%s\n' "$store" "$token" >"$envfile"
DATABASE_URL="postgres://foundation_api:${api_password}@${host}/lc_admin_fixture?sslmode=disable" LISTEN_ADDR=127.0.0.1:18081 go run ./cmd/api >/dev/null 2>&1 & api_pid=$!
sleep 1
printf 'Admin fixture API: http://127.0.0.1:18081\nLoad env: %s\n' "$envfile"
trap - ERR INT TERM
