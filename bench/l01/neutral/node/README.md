# Node W1 fixture adapter

The Node 24 command serves the [frozen W1 endpoint](../http/README.md) on IPv4
loopback using the [frozen PostgreSQL contract](../pg/README.md). It authenticates
the original bytes before parsing or acquiring a PostgreSQL connection. One
pooled client runs each READ COMMITTED transaction. SQL resolves scope and
writes the Inbox and intent or unscoped quarantine. A digest conflict returns
409. A failed admission or commit returns 503. A success ACK follows COMMIT.

The standalone pnpm lock pins node-postgres for transactions and jsonc-parser
for rejecting duplicate JSON member names. Install Node 24 and use the
repository's pnpm 10.33.0. From the repository root:

```sh
pnpm --dir bench/l01/neutral/node --ignore-workspace install --frozen-lockfile --ignore-scripts
pnpm --dir bench/l01/neutral/node test
```

## Run the listener

Use the [frozen SQL setup](../pg/README.md#checks) on a fresh disposable PG18
cluster and enable the ingress and reader logins with separate disposable
credentials. Set `PGPORT` to the published cluster port and supply the
**ingress** password via `PGPASSWORD` or a PostgreSQL passfile. The Node
listener uses the fixed database `lc_neutral_w1_test` and ingress role
`l01_w1_ingress`.

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 node bench/l01/neutral/node/main.mjs 19011
```

Run the [frozen wire and restricted reader checks](../http/README.md#observable-outcomes-and-sql-readback)
with the appropriate mode and reader credentials.

## Run the PG18 correctness tests

Use an integrator-owned disposable PG18 cluster with an empty database named
`lc_neutral_w1_test`. Set `PGPORT` and supply the disposable **administrator**
password via `PGPASSWORD` or a passfile. For the repository's container runtime,
set `LC_W1_PG_CONTAINER` to the owned container name so the frozen oracle can
run its `psql`. Otherwise use a PostgreSQL 18 `psql` on PATH.

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 PGUSER=postgres LC_W1_NODE_MODE=base \
  pnpm --dir bench/l01/neutral/node test:pg
```

Repeat with `LC_W1_NODE_MODE=extended` on a separate fresh cluster because the
fixture roles are cluster-wide. The tests use the listener's normal startup
function on an OS-assigned loopback port. Base expects 100 wire responses and
60 committed pairs. Extended expects 107 responses, 61 pairs and two
decryptable quarantine envelopes. After the frozen readback, the extended
run snapshots fresh quarantine envelopes before exact repeat. Both modes
check privilege denials, failed COMMIT, held COMMIT with concurrent exact and
conflicting deliveries, and an aborted response followed by exact redelivery.
Tear down each disposable cluster after its run.

The tests cover fixture correctness. Performance, providers, production
and the language comparison require separate evidence.
