# Go W1 fixture adapter

This command serves `POST /__l01/w1/webhook` on literal IPv4 loopback. It
authenticates the original bytes and checks the single-message shape before
opening a transaction. App, object and signing key come from the synthetic
corpus constants. PostgreSQL resolves the scope. On an unbound or inactive
asset, each delivery seals the exact wire body in the same transaction that
writes quarantine. An exact replay keeps the first committed envelope.

A successful commit for `routed`, `replay` or `quarantined` returns 200 and
`EVENT_RECEIVED`. Digest conflict returns 409. Bad signatures return 403,
invalid signed payloads return 400, and oversized bodies return 413.
Persistence failures, including failed COMMIT, return 503.

## Run the adapter

Use the frozen [PostgreSQL fixture setup](../pg/README.md#checks) on an empty,
disposable PostgreSQL 18 cluster. The fixture administrator must enable login
for the schema's `l01_w1_ingress` and `l01_w1_reader` roles with separate
disposable credentials. From the repository root, set `PGPORT` to the cluster's
published loopback port and supply the **ingress** password through
`PGPASSWORD` or a PostgreSQL passfile.

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 go run ./bench/l01/neutral/go 19011
```

The argument is the HTTP port. The command uses the fixed database
`lc_neutral_w1_test` and the fixed login `l01_w1_ingress`. In a second
terminal, run the frozen [wire and readback commands](../http/README.md#observable-outcomes-and-sql-readback)
with the same database and the separate reader credentials.

## Reproduce correctness checks

```sh
go test -count=1 -timeout=30s -v ./bench/l01/neutral/go
```

The default run uses scripted transactions to check zero persistence calls on
rejected requests, response ordering and the outcome of a failed commit. It
checks fresh quarantine UUIDs and nonces on repeat. `TestHTTPPostgres` skips
without an explicit mode.

For each mode, the integrator supplies a fresh cluster from the existing
ephemeral PG runtime and creates an empty database named
`lc_neutral_w1_test`. The fixture's role names are cluster-wide. Supply its
published `PGPORT`, and set `PGPASSWORD` to the disposable **administrator**
password. With the repository runtime, `LC_W1_PG_CONTAINER` is its existing
container name and the frozen JS PG check runs that container's `psql`. A
fixture with a local PostgreSQL 18 `psql` can omit this variable.

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 PGUSER=postgres LC_W1_GO_MODE=base \
  go test -count=1 -timeout=70s -run '^TestHTTPPostgres$' -v ./bench/l01/neutral/go
```

Repeat with `LC_W1_GO_MODE=extended` on the next fresh cluster. The test invokes
the frozen SQL setup, enables the two restricted logins with different
generated passwords and checks representative privilege denials. It starts the
same HTTP handler and ingress pool as the command. The frozen JS wire client
sends 100 base or 107 extended deliveries. The frozen JS PostgreSQL reader
checks 60 base or 61 extended Inbox/intent pairs and authenticates both
quarantine rows in extended mode. An additional staged retry checks that the
stored envelopes stay identical.

After the exact corpus snapshot, a deferred trigger forces a real COMMIT
failure and the reader checks for zero surviving rows. In the concurrent
case, the first request waits at COMMIT while exact and changed-digest
deliveries wait on the same PostgreSQL key. The test checks for no reply or
visible pair before release. After release it requires two exact ACKs, one
409 and one durable pair with the original digest. The faults use two
additional MIDs and the fixture is disposable after this test. The integrator
owns cluster teardown, harness wiring and independent review.

These checks target fixture correctness. A lost response after commit,
performance, the language comparison and product acceptance require separate
evidence.
