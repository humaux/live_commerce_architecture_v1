# Rust W1 fixture adapter

The loopback listener authenticates the original bytes, validates one message
and calls the frozen PostgreSQL admission function on one pooled connection per
READ COMMITTED transaction. The pool admits at most eight connections and
checks the restricted ingress identity on each new connection. Missing or
inactive bindings get a fresh UUID and AES-256-GCM envelope on each attempt.
The function retains the first committed envelope. A 200 or 409 response
requires a successful COMMIT. An admission or commit error yields 503.

## Direct tests and listener

Run from the repository root using the standalone Cargo lockfile. Set
`CARGO_HOME`, `CARGO_TARGET_DIR` and `TMPDIR` to the task's external volume
when running locally.

```sh
crate=bench/l01/neutral/rust/Cargo.toml
cargo test --locked --manifest-path "$crate"
```

Use a disposable PostgreSQL 18 fixture with the trusted bindings already
seeded. Set `PGPORT` explicitly and supply the disposable ingress login's
password in `PGPASSWORD`. The listener fixes the role, database and address.
[Fixture setup](../pg/README.md#checks)

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 \
  cargo run --locked --manifest-path "$crate" -- 19011
```

## Fresh PG18 conformance

Provide Node 24, an owned fresh PG18 cluster, an empty database named
`lc_neutral_w1_test` and the disposable postgres password in `PGPASSWORD`.
Supply a PG18 `psql` on PATH or set `LC_W1_PG_CONTAINER` to the matching
test container for the frozen SQL command. Set `PGPORT` to the cluster's
published loopback port in both cases. The runner checks freshness and creates
separate ephemeral ingress and reader logins.

```sh
LC_TEST_DATABASE_ALLOWED=1 PGHOST=127.0.0.1 PGUSER=postgres LC_W1_RUST_MODE=base \
  cargo test --locked --manifest-path "$crate" -- --ignored --nocapture
```

Run the same command with `LC_W1_RUST_MODE=extended` on a **second fresh
cluster**. Each run starts the normal listener on an OS-assigned loopback port,
runs the [frozen HTTP client](../http/check.mjs), then executes the matching
[frozen SQL readback](../pg/check.mjs) as the reader. Base checks 100 replies
and 60 pairs. Extended checks 107 replies, 61 pairs and two decryptable
quarantines, then snapshots envelopes across exact redelivery.

The additional fault cases fail a deferred trigger during COMMIT and hold
another COMMIT while an exact and a conflicting delivery both wait on the
same key. The reader checks pair visibility before and after release. In the
disconnect case, the client closes its HTTP/1 connection while PostgreSQL
is waiting in COMMIT and before the client receives response headers. The
reader must see the original pair after release **before** exact redelivery.
A disconnect after a completed COMMIT is outside that timing control.
