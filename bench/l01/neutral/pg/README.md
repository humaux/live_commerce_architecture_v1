# W1 neutral PostgreSQL contract proposal

Integrator review precedes adapter implementation. The benchmark keeps the single-message
`page` workload from [the portable corpus](../w1_corpus.json), following
[PLAN §3](../../../../experiments/PLAN.md) and
[I01, I02, I04](../../../../contracts/invariants.json). The
[product Inbox fixture](../../../../tests/foundation/meta_inbox_test.go) and
[product DDL](../../../../migrations/0028_meta_inbox.sql) supply the reference cases
for trusted routing, quarantine and rollback.

## Adapter boundary

- The fixture registrar seeds `l01_w1.binding` by trusted app, object and asset.
  Registrations stay fixed throughout a run. Use a schema owner for setup and a
  distinct ingress login. Grant ingress `USAGE ON SCHEMA l01_w1` and
  `EXECUTE ON FUNCTION l01_w1.admit(text,text,text,text,text,uuid,text,bytea,bytea)`.
  Reserve binding and table privileges for setup and readback.
- Verify HMAC-SHA256 over the exact request bytes before decoding the single
  message. Derive asset and MID from the authenticated body, verify entry and
  recipient agreement, and take app and object from server configuration.
  Discard the supplied tenant header. Bad signatures make zero persistence calls.
- In a PostgreSQL 18 READ COMMITTED transaction, call
  `SELECT l01_w1.admit($1,$2,$3,$4,$5)` with app, object, asset, MID and lowercase
  SHA-256 of the exact body. The function resolves active scope from the trusted
  registry and copies tenant, store and binding to the Inbox. The dedup key is
  `(app, object, asset, tenant, store, MID)`. The first receipt creates one
  linked neutral intent. `replay` preserves the existing pair. A different digest
  at that key returns `conflict` with no write and maps to HTTP 409.
- `needs_quarantine` requires a second call in the same transaction, appending
  `(id, key_id, nonce, ciphertext)` to the argument list. Seal the exact wire body
  first using the [product v1 AEAD layout](../../../../internal/integrations/meta/payload.go)
  with class `quarantine`, a fresh UUID, a random 12-byte nonce and AES-256-GCM
  ciphertext followed by the 16-byte tag. For this benchmark, the AAD EventKey is
  SHA-256 of the compact JSON array `[app,object,asset,MID]` and PayloadHash is the
  body digest. BodyHash, tenant, store and route are empty and route epoch is zero.
  Use the synthetic fixture key from `check.mjs` only for this workload. SQL stores
  the opaque envelope in an unscoped table with no intent, deduping on app, object,
  asset, MID and digest. Encryption authenticity is checked by decrypting the
  committed envelope in readback. Dynamic registration, production normalization,
  key rotation, payload retention and product worker execution remain outside
  this benchmark.
- ACK only after a successful commit with `routed`, `replay` or `quarantined`.
  Roll back on errors. A lost ACK permits an exact redelivery under the same
  fixture. A returned status before commit is provisional.

## Checks

Offline, from the repo root:

```sh
node --check bench/l01/neutral/pg/check.mjs
node bench/l01/neutral/pg/check.mjs oracle
```

For the integrator CI, use a fresh project-approved disposable PostgreSQL 18
instance with an empty `lc_neutral_w1_test` database and an applicable test lease.
Supply `PGPORT`, `PGUSER` and credentials through libpq environment or passfile.
Run from the repo root:

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 node bench/l01/neutral/pg/check.mjs smoke
```

`smoke` creates the schema and seeds the trusted fixtures, verifies the 100
wire signatures in the test driver, and sends the 80 authenticated inputs to
SQL. It checks 60 committed pairs, lost ACK replay, same MID in a second
authorized store, changed-body conflict, unknown and inactive quarantine, and an
injected intent failure that must leave no half-write. A separate read-only
PostgreSQL connection checks exact row multisets and decrypts the quarantine
rows. The four readback negative controls mutate observations in memory:
missing pair, duplicate pair, spoofed tenant replacing the true tenant, and
Inbox without intent. Their rejection checks the comparator; the injected intent
failure checks the database transaction. Use the command exit code for the full
run. Tear down the disposable instance after the run.

Future adapter tests can use `setup` on a fresh database, send the full corpus
through HTTP, then use `readback`. Use `readback-extended` after also exercising the
supplemental fixtures in `check.mjs`. HTTP status, ACK order, database role grants
and concurrent replay need adapter-level verification. `oracle` is offline
evidence. Only a successful `smoke` invocation supplies real-PG evidence for
this SQL slice. W2, load or quota runs, and full T19/G12/G15 are outside its scope.
