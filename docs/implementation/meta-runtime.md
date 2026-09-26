# Meta receive/consume runtime

Status: **ACCEPTED_LOCAL_PRIVATE_RUNTIME**. This is a local runtime
increment over the accepted inbox and social consumer. It does not enable a
customer callback, authorize sending, or establish production readiness.
Configuration and gates: [original contract](../../contracts/meta-runtime-v1.md).
The failed shared-schema candidate is retained in the acceptance record.
The [isolation revision](../../contracts/meta-runtime-isolation-v1.md) is
implemented through `d702bb1`; frozen source/tests `593291e` passed 552 full
PG/race/vet tests and three same-source browser gates with independent review.
See the [exact evidence and retained failures](2026-09-26-meta-runtime-acceptance.md).
This accepts the local receive/consume boundary, not customer activation.

## Call and ownership map

| Entry | Uses | Authority / lifetime |
| --- | --- | --- |
| `cmd/api` Meta assembly | `LoadWebhookEndpoints`, `LoadPayloadKeyring`, `platform.OpenMetaIngressPool`, `NewWebhookRouter` | Dedicated ingress login; API owns pool and HTTP shutdown; never starts a worker |
| `NewWebhookRouter` | `NewInbox` → `NewInboxHandler` → existing strict signature/body verifier and receipt transaction | Exact configured raw path; ACK only after receipt/ciphertext/job COMMIT |
| `cmd/meta-worker` | `platform.OpenMetaWorkerPool`, `OpenMetaConsumerPool`, `LoadPayloadKeyring`, `NewConsumerClient`, `jobqueue.Run` | Dedicated Meta lifecycle and projection logins; caller owns both pools; shared signal/start/drain/cancel code |
| `NewConsumerClient` / `NewInbox` | existing `ConsumerWorker` / insert-only River client | Both use fixed `river_meta`; no schema setting or old-lane fallback |
| Lifecycle pool preflight | shared `validatePoolAuthority` | `commerce_meta_worker` only; checks direct, PUBLIC, column, inherited and SET-reachable object privileges as well as role/owner authority |
| Both startup paths | `platform.ValidateSameDatabase` and `meta_inbox.runtime_ready()` | No permanent probe records or extra table access; fail before listen/fetch |

There are no new modules or services. Existing Go standard-library JSON,
cryptography, HTTP and context facilities plus pgx/River are reused. Payload
keys are a separate keyring from merchant account credentials. The worker must
never receive app secrets or webhook verification tokens.

## Startup and failure boundaries

Both enable flags default off. A disabled component reads only its own flag.
Enabling the API component first validates the effective literal-loopback
listener, then reads Meta configuration. This prevents accidentally exposing
the private increment by changing a deployment's bind address.

Each router/client constructor has one shared five-second preflight; Meta
assembly has ten seconds, inheriting earlier cancellation. These construction
contexts do not become River's running context. API/worker processes close
owned pools after partial assembly failures. Transaction-probe cleanup gets an
independent two-second context per transaction, even after caller cancellation.

The API's merchant pool and ingress pool must reach the same database; likewise
the worker and consumer pools. The probe holds a cryptographically random
transaction advisory lock through one pool and checks conflict through the
other. Same names/hosts, matching row IDs and cloned data are insufficient.
This assumes fixed routing to trusted PostgreSQL servers; it cannot guarantee
future administrative retargeting. Both transactions roll back before return.

`0030_meta_runtime.sql` remains checksum-stable. New `0031` prepares the isolated
schema/NOLOGIN role and forces readiness false. `migrations.Apply` runs native
River migrations for both schemas, then `post_river/0004` moves completed-linked
Meta jobs with IDs, every job field, paused queue state and sequence high-water
preserved. Its final transaction installs new guards and grants before readiness
can pass. Readiness now checks five guards, including rejection on the old lane.
Running/poisoned rows, unexpected destination contents or lock contention stop
cutover. A failed post phase may leave preparation/upstream ledgers committed;
readiness stays false until a safe retry. See the frozen contract for drain and
rollback semantics; none of these steps has been executed in customer production.

## Diagnosis and maintenance

Queue selection is not a maintenance boundary. In pinned River v0.40.0,
`client.go` constructs the schema-wide leader, scheduler, rescuer and cleaner
for every work-capable client. A real Meta-only process promoted an unrelated
payment job `scheduled` → `available`, attempt 0 unchanged. The upstream
rescuer also discards stale running kinds absent from that client's worker map.
Do not mask this by moving fixture deadlines, pre-promoting jobs or relying on
jitter. [Failure record](2026-09-26-meta-runtime-acceptance.md) retains both the
initial root failure and a deterministic reproduction. Existing payment/expiry
clients sharing `river` require their own isolation audit; moving Meta alone
does not certify those workers as safe to run together.

Errors are fixed safe codes. Do not turn on raw driver/River configuration logs
or print environment values to debug an unsuccessful startup. Inspect which
bounded stage failed in an isolated environment, then check role separation,
same-database routing, applied migrations and exact guard metadata. A retryable
missing decryption key is not permission to discard or replay an external event.

Changing runtime wiring, schema guards, PG/pgx/River, the strict parser or secret
loader requires:

1. `go test -race ./internal/platform ./internal/integrations/meta ./cmd/api ./cmd/meta-worker`
2. `bash scripts/dev/test-local.sh --meta-runtime`
3. `bash scripts/dev/test-local.sh --meta-inbox` and `--meta-consumer`
4. `bash scripts/dev/test-local.sh` (complete real PG/race/vet), then independent review.

The fixture runner creates its own labelled, loopback-only disposable PG and
removes only that verified fixture. It must not reuse production `DATABASE_URL`.
Historical 0028→0029 and 0029→0030 tests retain the original schemas and SQL.
Separately test populated 0030→0031/post0004 and repeated Apply; a clean install
alone is not upgrade evidence. MIso01–05 also require actual scheduler, rescuer
and cleaner activity in both lanes, with unchanged opposite-lane full rows.

Production secret provisioning/rotation, trusted OAuth route registration,
public proxy/TLS, social read UI, retention and outbound consent/window handling
are separate increments. Do not use localhost test output as Meta approval or
mark the whole SaaS deployable from these gates.
