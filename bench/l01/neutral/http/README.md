# L01 W1 fixture HTTP conformance candidate

This contract gives future Go, Rust and Node adapters the same wire input and
response checks. It uses the [signed corpus](../w1_corpus.json),
[neutral PostgreSQL contract](../pg/README.md), [L01 plan §3](../../../../experiments/PLAN.md)
and the [Go pilot observer's HTTP conventions](../../go-w1/observer.go).

## Endpoint and trusted configuration

Run a dedicated fixture listener on literal IPv4 loopback with an explicit
port. Expose only `POST /__l01/w1/webhook` for this contract. Keep the listener
out of product routing and use a fresh disposable PostgreSQL 18 fixture
database per run. The launcher fixes the app ID, object and synthetic HMAC
secret to `app_id`, `object` and `hmac_secret` in the checked-in corpus. Use the
ingress login and fixture quarantine key specified by the SQL contract.
Keep database credentials in the environment or a passfile. Only the fixture
administrator seeds the trusted bindings. Run the reader independently.

The client sends each `raw_body` as its original UTF-8 bytes, with
`Content-Type` from `content_type`, `X-Hub-Signature-256` from `signature_header`
and `X-Tenant-ID` from `untrusted_tenant_header`. It sends the actual byte
count in `Content-Length`. Classification and expected results stay in the
client. The server ignores the spoofed tenant header and gets tenant and store
from the registry. Verify HMAC-SHA256 over the wire bytes before parsing
controlled JSON. Require one entry and one message, configured object
agreement and entry/recipient agreement. Derive asset and MID from the
authenticated body and use the exact wire digest in SQL. Follow the SQL
contract for the transaction, quarantine seal and scope.

## Observable outcomes and SQL readback

| Request | HTTP expectation | Persistence expectation |
| --- | --- | --- |
| First valid delivery to an active binding | 200, exact body `EVENT_RECEIVED` | One committed Inbox and linked intent in the registered scope |
| Byte-identical valid repeat | 200, same exact ACK | Existing pair, zero additional rows |
| Bad signature | 403 | Zero persistence calls |
| Same MID under the second active binding | 200, same exact ACK | One pair in the second registered scope |
| Same scoped key with a changed, correctly signed body | 409 | Original pair retained, zero additional rows |
| Missing or inactive binding, including an exact repeat | 200, same exact ACK | Unscoped encrypted quarantine, no Inbox or intent. Seal each attempt freshly and retain the first committed envelope. |

The server emits the 200 ACK only after successful commit. Non-200 response
bodies have no semantic assertion in this harness. The client requires a
complete reply of at most 512 bytes within three seconds per request. It
sends one request at a time in persisted order, with no redirects or retries.
A mismatch exits nonzero at the first failed case with its ordinal and class.
Later cases are unattempted.

Use [SQL setup](../pg/README.md#checks) before running the HTTP command. Run the
corresponding SQL readback as `l01_w1_reader` against the same disposable database
after the HTTP run. Choose one row per fresh fixture.

| HTTP command from repo root | SQL command suffix | Expected durable state |
| --- | --- | --- |
| `node bench/l01/neutral/http/check.mjs <port>` | `readback` | 60 exact Inbox/intent pairs and no quarantine |
| `node bench/l01/neutral/http/check.mjs <port> --extended` | `readback-extended` | 61 exact pairs and two decryptable quarantine rows |

The SQL command is `node bench/l01/neutral/pg/check.mjs <suffix>` with the
disposable database opt-in and loopback PG settings from the SQL contract.
Extended mode first sends all 100 persisted cases. It then uses the same MID
and wire shapes as the SQL smoke for a second-store delivery, exact redelivery
of the first authenticated request, changed digest and two deliveries each to
the missing and inactive bindings.

`HTTP_RESPONSES_CHECKED` records wire replies only. Both successful HTTP
and independent real PostgreSQL readback are required for durable outcome
evidence. A final snapshot can detect an extra or absent row or changed digest.
It cannot prove there were zero database calls for rejected signatures, that
a repeat returning a valid ACK traversed persistence, or that a quarantine
repeat kept the original envelope without a pre-repeat snapshot. Verify those
properties with adapter instrumentation or staged readback as appropriate.
Commit-before-ACK requires control over the adapter's real commit boundary.
Hold the transaction before commit and observe that no ACK arrives, then
release it and check ACK plus reader visibility. Inject a failed commit and
check that no success ACK or partial rows survive. Run concurrent replay and
lost-ACK fault cases separately. The sequential client has no fault controls.

## Local synthetic controls

```sh
node --test bench/l01/neutral/http/check.test.mjs
```

The tests use ephemeral loopback response stubs to assert bytes, headers and
positive replies, then verify that the CLI exits 1 for a wrong status, a
duplicate with an empty ACK and a duplicate with no response. These controls
establish harness behavior. Adapter execution, PostgreSQL readback and
commit-order testing are `NOT_RUN` in these tests.
