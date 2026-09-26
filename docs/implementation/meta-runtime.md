# Meta receive/consume runtime

Status: **FAILED_MR04 / REVISION_REQUIRED**. This is a local runtime
increment over the accepted inbox and social consumer. It does not enable a
customer callback, authorize sending, or establish production readiness.
Configuration and gates: [original contract](../../contracts/meta-runtime-v1.md).
This page maps the failed candidate, not a production-safe queue boundary.
The [isolation revision](../../contracts/meta-runtime-isolation-v1.md) must be
implemented and independently accepted before enabling this runtime.

## Call and ownership map

| Entry | Uses | Authority / lifetime |
| --- | --- | --- |
| `cmd/api` Meta assembly | `LoadWebhookEndpoints`, `LoadPayloadKeyring`, `platform.OpenMetaIngressPool`, `NewWebhookRouter` | Dedicated ingress login; API owns pool and HTTP shutdown; never starts a worker |
| `NewWebhookRouter` | `NewInbox` → `NewInboxHandler` → existing strict signature/body verifier and receipt transaction | Exact configured raw path; ACK only after receipt/ciphertext/job COMMIT |
| `cmd/meta-worker` | `platform.OpenWorkerPool`, `OpenMetaConsumerPool`, `LoadPayloadKeyring`, `NewConsumerClient`, `jobqueue.Run` | Separate lifecycle and projection logins; caller owns both pools; shared signal/start/drain/cancel code |
| `NewConsumerClient` | existing `ConsumerWorker` | Fetches only `meta_inbox`, but River maintenance still covers the shared `river` schema; MR04 failure |
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

`0030_meta_runtime.sql` adds only a boolean startup predicate and the worker's
schema USAGE needed to call it. Explicit table/function revocations remain.
It validates all four queue/social guards and active reserved jobs, not just an
empty queue. PL/pgSQL defers resolving River's table until after River migrations
on fresh installations. Do not move this into an SQL-language creation-time
reference or rewrite the prior migration ledger.

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
For a populated upgrade, preserve the actual 0029 checksums/data and apply 0030
twice; a clean install alone is not upgrade evidence.

Production secret provisioning/rotation, trusted OAuth route registration,
public proxy/TLS, social read UI, retention and outbound consent/window handling
are separate increments. Do not use localhost test output as Meta approval or
mark the whole SaaS deployable from these gates.
