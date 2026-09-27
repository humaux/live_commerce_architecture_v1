# T08 independent media process runtime — LMW01–05

Status: **IMPLEMENTED_PENDING_LOCAL_ACCEPTANCE** (2026-09-27),
base `595ca7c`, draft `ed0fe57`. Independent static security/testability preflight
task `1983ed1e-2369-41ef-a821-4b205548015b` found no confirmed blocking P0/P1.
Interface frozen at `6b69a25`; source `6bbe24c` is integrated as `fda7082`.
Independent fixed-source static review `66b16a27-340a-4692-bc3d-7e764cacd81d`
found no confirmed scoped P0/P1. LMW01–05 process acceptance remains NOT_RUN.
This wires the accepted [LME](live-media-execution-v1.md) and
[LMR](live-media-stop-v1.md) into a runnable command. It does not grant LIVE
authority, add a public control route or count as real Cloud/G06 acceptance.

## Reuse and ownership

Add `cmd/media-worker`; follow `cmd/meta-worker` for signal handling, fixed safe
errors, caller-owned pools and `jobqueue.Run`. Reuse `live.NewMediaClient`,
`platform.OpenMediaWorkerPool`, `OpenMediaExecutorPool`, same-database proof,
the isolated `river_media` native lifecycle and existing Start/Query/Stop logic.
No migration, dependency version, queue, table, generic dispatcher, lease or
provider retry is added. No API startup or administrative production migration.

The platform-owned local MOCK transport is explicit configuration, never a
merchant URL or network proxy. This slice must not let a MOCK flag contact real
Cloud. Actual LIVE provisioning and dispatch remain a separate authority gate.

## Fixed configuration

`COMMERCE_MEDIA_WORKER_ENABLED`: absent/empty/`0` returns success after reading
only this flag. Only `1` enables; all other forms fail. No fallback `DATABASE_URL`,
merchant account, Meta app, payment or identity secrets may be read.

Enabled inputs:

| Variable | Validation / meaning |
| --- | --- |
| `COMMERCE_MEDIA_WORKER_DATABASE_URL` | Nonblank, 1..8192 bytes; admitted media lifecycle login |
| `COMMERCE_MEDIA_EXECUTOR_DATABASE_URL` | Same bounds; separately admitted executor login |
| `COMMERCE_MEDIA_WORKER_CONCURRENCY` | Empty defaults 4; canonical integer 1..32 |
| `COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID` | Existing LKM key-ID grammar |
| `COMMERCE_MEDIA_MATERIAL_KEYS_JSON` | At most 8192 bytes; exact `{"keys":[{"id":"...","key_base64":"..."}]}`; 1..16 distinct IDs, canonical 32-byte standard-base64 values; active ID must exist |
| `COMMERCE_MEDIA_PROJECTS_JSON` | At most 65536 bytes; exact `{"projects":[...]}`; 1..128 items with fields below |

Each project has exactly `project_id`, `credential_version`, `endpoint`,
`api_key`, `api_secret`, `stream_hosts`, `mock_dial_address`, `mock_ca_pem`.
Project ID uses existing `^[A-Za-z0-9_-]{1,80}$`, credential version is a positive
int64 JSON integer (not a string, fractional/exponent form or lossy float).
Duplicate (project ID, version) is rejected. Existing LKP config validation
governs endpoint, API credentials and stream-host allowlist. Environment is
internally fixed `MOCK`; no user-provided environment field is accepted.

`mock_dial_address` is canonical literal `127.0.0.1:<1..65535>` or
`[::1]:<1..65535>`, never a hostname, wildcard, zone, proxy or remote address.
`mock_ca_pem` is nonempty, bounded to 16384 bytes and must load certificate PEM
into a fresh `x509.CertPool`. Require TLS >=1.2, normal certificate-chain and
hostname verification for the configured endpoint, with no InsecureSkipVerify.
Use a private `http.Transport` per project, `Proxy:nil`, disabled keep-alives,
and a bounded dialer hard-pinned to the validated loopback address. No default
transport/proxy/DNS fallback or process-global TLS mutation. Client redirects
remain disabled. A process restart is required for configuration/key rotation;
retain previous IDs/versions while existing frozen attempts need them.

Reuse the existing LiveKit strict bounded object decoder within that package
for these two JSON inputs; reject duplicates, unknown/missing fields, malformed
types, trailing data and invalid UTF-8. Loader output and intermediate exported
secret-bearing types redact String/GoString/JSON representations. Never return
raw parse, driver, key, URL, transport or provider errors to the command log.

Implementation seam: `livekit.LoadWorkerEnvironment(getenv func(string) string)
(WorkerEnvironment, error)`. Its result contains `Keys *MaterialKeyring` and
`Projects []WorkerProject`; each project has `ProjectID string`,
`CredentialVersion int64`, `Config Config`, `Transport http.RoundTripper`.
The command translates these directly to the existing `live.MediaProject`;
the domain packages do not depend on the command or on each other cyclically.

## Lifecycle and errors

Validate enabled configuration before network/DB work. `run(ctx,getenv)` rejects
nil context when enabled. One ten-second parent startup context spans opening
both pools and constructing `NewMediaClient` (its own five-second preflight
inherits the earlier deadline); no constructor context becomes the run context.
Close every successfully opened pool on partial failure and after shutdown.
After assembly, use existing `jobqueue.Run` startup watchdog and 15s graceful /
5s forced stop. SIGINT/SIGTERM stop this process only; **never issue provider
Stop solely because the worker is shutting down**. Durable recovery resumes
the original operation after restart; no new Start or reset Stop budget.

Log readiness only as fixed `media_worker_ready`, after actual River start.
Error codes: `media_worker_invalid_config`, `media_worker_database_unavailable`,
`media_worker_start_failed`, `media_worker_stop_failed`. Ready is local startup,
not Cloud, stream health or production approval. No health server needed here.

## Independent acceptance gates

| Gate | Causal evidence |
| --- | --- |
| LMW01 config and secrets | Actual command disabled reads flag only / exits zero; malformed enabled inputs fail closed; all secret-bearing values redact; canonical boundaries, duplicate/unknown fields, key rotation and explicit endpoint pairs covered |
| LMW02 transport | Real local TLS positive with endpoint-name certificate; invalid CA/wrong hostname fails, remote/DNS/wildcard dial rejected; hostile proxy env gets zero requests; redirects cannot escape; no default transport changes |
| LMW03 startup authority | Real PG and actual command reject wrong roles, mixed authority, cross-DB pools and readiness drift before ready/work; partial startup closes connections; existing LME/LMR ACL/legacy admission gates remain intact |
| LMW04 process execution | Build actual command, start against task-owned PG/local TLS with durable prepared attempt, observe one Start plus persisted observation, request Stop through existing authorized planner, prove associated terminal evidence and unchanged other-queue rows; never call `NewMediaClient` as a substitute for this process |
| LMW05 shutdown/recovery | Signal actual ready process, assert timely exit and released PG connections; restart against same persisted attempt, no replacement Start or reset Stop budget; targeted missing-key/project failure retains uncertainty without provider side effects and restart with original configuration can resume |

Independent test paths: command package tests and foundation process tests;
owned synthetic credentials only. Extend `scripts/dev/test-local.sh` with a
focused `--live-media-runtime` selector, preserving old gates and individual
deadlines. Root reruns focused and full PG/race/vet on the final integrated tree.
Record command, exit, counts, SHA, failed attempts and fixture cleanup. No UI
changes here; browser gates become mandatory with the subsequent HTTP/studio.

## Stop line and upgrade signal

Independent implementation review remains required; unresolved P0/P1 blocks integration.
No hidden relaxing of DB admission or TLS verification to make the process run.
LIVE requires trusted credential/destination provisioning and real provider
tests; HTTP/BFF needs current merchant authorization and stable read DTOs;
studio requires approved visual layout and desktop/mobile acceptance. Those
remain necessary for the requested deployable product, not replaced by MOCK.
