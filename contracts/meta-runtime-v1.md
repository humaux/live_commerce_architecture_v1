# Meta receive/consume runtime v1

Status: **FROZEN / IMPLEMENTATION_REQUIRED**. Builds on accepted MI01–07 and MC01–07;
not authorization to configure customer Meta callbacks or deploy publicly.
Base `5523826`. This makes the existing components executable, not a new broker
or social microservice. API and worker remain the same Go modular monolith.

## Reuse and ownership

- Reuse `cmd/api` HTTP lifecycle; mount Meta outside ServeMux path cleaning.
  Use a dedicated ingress pool, never the merchant/API pool.
- Add `cmd/meta-worker`, borrowing the existing `jobqueue.Run` startup watchdog,
  signal/drain/cancel behavior. It opens distinct ordinary worker and Meta
  consumer pools and closes both on every return path.
- Reuse `NewInbox`, `NewInboxHandler`, `NewConsumerWorker`, `NewVerifier`,
  `NewPayloadKeyring`, strict JSON/Unicode validation and River `meta_inbox`.
  No dependency, queue, login/tenant authority, provider call or sending feature.
- Integrator owns `0030_meta_runtime.sql`, platform pool-open wrappers and the
  contract. Go author owns runtime/environment code, API wiring and CLI. Test
  author owns independent real-process/PG tests. Freeze before concurrent work.

## Configuration contract

All errors/logs are fixed safe codes; no DSN, raw environment, body, signatures,
verify tokens or secrets in diagnostics. Secret-bearing config structs format
and JSON-encode as redacted. Environment injection is the current server-side
interface; it does not claim production vault/rotation automation.

| Setting | Rule |
| --- | --- |
| `COMMERCE_META_WEBHOOK_ENABLED` | absent/`0` disabled; `1` enabled; other values invalid |
| `COMMERCE_META_INGRESS_DATABASE_URL` | nonempty, at most 8192 bytes; dedicated ingress login |
| `COMMERCE_META_APPS_JSON` | at most 32768 bytes; exact object `{ "apps": [{"app_id":"…","object":"page or instagram","app_secret":"…","verify_token":"…"}] }`; 1–16 entries; duplicate `(app_id,object)` rejected |
| `COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID` | existing payload key-ID rules |
| `COMMERCE_META_PAYLOAD_KEYS_JSON` | at most 8192 bytes; exact object `{ "keys": [{"id":"…","key_base64":"…"}] }`; 1–16 distinct IDs, canonical padded standard base64 of 32 bytes; existing nonzero/distinct-key rules |
| `COMMERCE_META_WORKER_ENABLED` | absent/`0` disabled; `1` enabled; other values invalid |
| `COMMERCE_META_WORKER_DATABASE_URL` | nonempty, at most 8192 bytes; ordinary dedicated worker login |
| `COMMERCE_META_CONSUMER_DATABASE_URL` | nonempty, at most 8192 bytes; dedicated Meta consumer login |
| `COMMERCE_META_WORKER_CONCURRENCY` | absent defaults to 4; canonical integer 1–16 |

Disabled components read only their own enable flag, no secret/DSN/key settings
and create no pools/listeners/workers. API checks its effective `LISTEN_ADDR`
with the existing literal-loopback predicate **before** reading Meta authority
settings. Meta does not depend on merchant identity enablement. This increment
does not configure a public reverse proxy, TLS or trusted OAuth proof issuer.
There is no flag claiming that localhost equals provider approval.

JSON rejects duplicate member names, unknown fields, trailing values, malformed
UTF-8/unpaired surrogates, missing/wrong-type fields and bounds violations.
Reuse the existing strict object parser before typed validation. App/secret
validation delegates to `NewVerifier`; keys delegate to `NewPayloadKeyring`.
The worker reads payload keys but **never** app secrets/verify tokens.

## Frozen Go surfaces

```go
type WebhookEndpoint struct { Path string; Verifier *Verifier }
func LoadPayloadKeyring(getenv func(string) string) (*PayloadKeyring, error)
func LoadWebhookEndpoints(getenv func(string) string) ([]WebhookEndpoint, error)
func NewWebhookRouter(context.Context, *pgxpool.Pool, *PayloadKeyring, []WebhookEndpoint) (http.Handler, error)
func NewConsumerClient(context.Context, *pgxpool.Pool, *pgxpool.Pool, *PayloadKeyring, int) (*river.Client[pgx.Tx], error)
// Pools above: worker first, consumer second; caller retains both.
func platform.OpenMetaIngressPool(context.Context, string) (*pgxpool.Pool, error)
func platform.OpenMetaConsumerPool(context.Context, string) (*pgxpool.Pool, error)
func platform.ValidateSameDatabase(context.Context, *pgxpool.Pool, *pgxpool.Pool) error
```

Paths are derived only from validated config:
`/v1/meta/webhooks/{app_id}/{object}`. Different apps/objects have distinct
verifiers and cannot select tenants/stores. Router admission rejects nil/invalid
verifiers, forged paths and duplicates. It never trusts forwarded Host.
Only exact unescaped configured paths dispatch; escaped, trailing-slash,
dot-segment, duplicate-slash and unknown paths fail without redirect or body
admission. API reserves the Meta prefix before ServeMux, including paths whose
cleaned form would enter that prefix (classification only; never rewrite and
dispatch a cleaned path). Existing raw handler owns method/query/signature/body
limits and atomic ACK. API starts no River workers.

## Queue admission and SQL

`0030_meta_runtime.sql` adds only `meta_inbox.runtime_ready() RETURNS boolean`,
STABLE SECURITY DEFINER with `search_path=pg_catalog`, owned by the existing
NOLOGIN non-owner Meta writer; PUBLIC execution revoked, granted only to ingress
and ordinary worker roles. Use PL/pgSQL to defer River-table resolution until
after its own migrations on a fresh install. No table/credential access granted.

The predicate checks enabled exact `meta_job_family` BEFORE INSERT/UPDATE and
deferred `meta_job_commit` INSERT triggers: expected functions, type, ownership,
SECURITY DEFINER and deferral attributes. It also rejects active reserved-kind
or reserved-queue rows with wrong kind/queue, unique key, malformed/extra args,
or absent exact completed ROUTED event/job link. It returns only a boolean.
Historical terminal jobs never cause unrelated business jobs to be consumed.
No policy check scans/returns plaintext or grants callback callers SQL authority.

Both runtime router and worker constructors require readiness before listening
or fetching. Ordinary worker pool is validated separately from consumer pool.
The predicate also verifies both `social_message_commit` and
`social_comment_commit` on the exact social tables, pointing to
`meta_inbox.guard_social_insert()`: AFTER ROW INSERT, enabled O/A, deferrable
and initially deferred. All four guards require no WHEN/column/argument filter,
the expected NOLOGIN writer, SECURITY DEFINER and exact safe search_path; merely
checking trigger names is insufficient.

Pool names, host strings, database names and matching event UUIDs are not proof
of a shared database (restored clones can contain the same IDs). API's existing
main pool versus ingress, and worker versus consumer, must pass
`ValidateSameDatabase` before listening/fetching. It uses a cryptographically
random nonzero signed bigint, at most five seconds of probe work (independent
rollback budgets below are additional): transaction A must
acquire `pg_try_advisory_xact_lock`, then transaction B must fail to acquire the
same lock while A is held. Any other outcome/error fails closed. Both pools
remain caller-owned; each transaction uses an independent background two-second rollback before
return. No permanent marker, new table, widened grants or connection-string
comparison. This is configuration-coherence checking against trusted PG
servers, not attestation against a malicious database administrator/server.

API assembly and CLI pool-open/constructor phase each have one shared 10-second
startup deadline. `NewWebhookRouter` and `NewConsumerClient` each have one shared
five-second preflight covering role checks, same-database checking where
applicable, and readiness, inheriting any shorter caller deadline.
Do not pass an expiring constructor context as River's running context.
Client registers only `ConsumerWorker` on fixed `meta_inbox`, concurrency 1–16,
no periodic jobs/default queue. River's logger is discarded like existing
workers until an explicit redaction contract exists. `jobqueue.Run` owns start
and stop; caller closes pools. A fixed `meta_worker_ready` witness means local
startup only, never platform authorization.

## Executable gates

| Gate | Required actual evidence |
| --- | --- |
| MR01 | Disabled zero sensitive reads/effects; strict env/app/key/canonical-number bounds; unknown/duplicate/trailing/malformed JSON; redacted formatting and errors; worker never reads app credentials |
| MR02 | Real PG ingress/worker/consumer exact-role and swapped/mixed/owner negatives; distinct/clone DBs denied; missing/disabled/replaced/filter-altered River or social guard and poisoned active queue deny startup before claim; no queue fetch by API |
| MR03 | Real API binary + separate worker binary + isolated PG: signed Page and IG HTTP produce committed scoped social facts; duplicate HTTP remains one fact; wrong signature/path/method does not admit; unknown assets stay quarantined; existing API routes still behave as before |
| MR04 | Real worker startup/stop/signals, partial assembly closes pools, missing key leaves retryable pending source, worker restart processes retained work; unrelated payment/expiry/default jobs unchanged; bounded startup and no secret-bearing logs |
| MR05 | Fresh/repeated and populated 0029→0030 migration preserve receipts/routes/jobs/body; old MI/MC gates, full PG/race/vet and independent source/evidence review; fixture/process cleanup |

Existing MC upgrade test must keep its exact pre-0029 baseline but explicitly
account for the new 0030 migration, not assume forever that only one later
migration exists. Preserve all old data and authority assertions.

## Limits and upgrade signals

Local runtime acceptance is not public deployment. Trusted OAuth registration,
production secret management, social read UI, privacy/retention automation,
outbound windows/consent and live provider qualification remain separate work.
One configured app can route many merchant assets through the existing DB;
do not create one app/process per merchant. Limits above bound static app/key
config, not tenant count. Add automated app/secret lifecycle only when a real
rotation/onboarding workflow is specified; do not replace authenticated route
proof with environment tenant mappings or a test fixture in production.
The database probe assumes fixed trusted pool routing, not later dynamic
retargeting. PostgreSQL advisory locks are database-local and transaction locks
are released when their transaction ends; see the official
[pg_locks](https://www.postgresql.org/docs/18/view-pg-locks.html) and
[explicit locking](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS) documentation.

Independent preflight of `020bbd3` closed the two P1 findings; this freeze also
corrects the social guard schema and makes the constructor/rollback deadlines
explicit as requested. MR01–05 remain NOT_RUN for this runtime increment.
