# Media worker runtime and maintenance

Status: **PASS_LOCAL_MOCK_PROCESS_ONLY / LMW01–05 PASS**, contract frozen at `6b69a25`.
See [configuration and gates](../../contracts/live-media-runtime-v1.md).
The [runtime acceptance](2026-09-27-live-media-runtime-acceptance.md) proves this
command against local PG/TLS fixtures; it builds on the
[Stop acceptance](2026-09-27-live-media-stop-acceptance.md), not a deployed service.

## Call and ownership map (source `6bbe24c`, integrated `fda7082`)

| Caller | Existing dependency | Ownership / boundary |
| --- | --- | --- |
| `cmd/media-worker.run` configuration | `livekit.LoadWorkerEnvironment` | Read only media settings; disabled reads enable flag only; explicit synthetic/local transport config |
| Environment loader | LKP strict `decodeObject`, `livekit.New`, LKM `NewMaterialKeyring`; Go x509/TLS/net/http | Reject ambiguous input; independent keyring and transport per project/version; no global transport mutation |
| `run` assembly | `platform.OpenMediaWorkerPool`, `OpenMediaExecutorPool`, `live.NewMediaClient` | Command owns and closes both pools; one bounded construction deadline; existing ACL/readiness/same-DB checks run before start |
| `jobqueue.Run` | Native River client in `river_media` | Existing bounded start, signal/drain/cancel; only this process shuts down, no provider Stop on signal |
| Media worker | Existing LME/LMR material loading, exact Query and bounded Stop | Original operation/generation/lease and durable budget; only coherent provider terminal evidence closes resource responsibility |

No new package dependency, native schema or provider retry is required. The
small field translation from loader projects to `live.MediaProject` keeps the
LiveKit package independent of the business `live` package. Do not introduce
a shared service/config framework merely to remove that translation.

## Operational meaning

This command is off by default. Enabling it does not mint merchant grants,
provision destination eligibility, create media attempts or expose HTTP routes.
The current transport dials only a configured literal loopback TLS endpoint;
it verifies the endpoint hostname and the supplied CA. It cannot stand in for
LiveKit Cloud acceptance. Synthetic fixtures must issue matching certificates,
not turn off certificate verification to satisfy the gate.

Configuration and key rotation require restart. Retain original material keys
and project credential versions while frozen attempts still reference them.
Removing them retains uncertainty rather than authorizing a replacement Start
or a different endpoint. Restart continues the original durable work; a
shutdown signal is not a merchant Stop command.

## Diagnosis and upgrade checklist

| Safe error / symptom | Inspect in an isolated environment |
| --- | --- |
| `media_worker_invalid_config` | Exact enabled flag, bounded fields, canonical IDs/integers/base64, certificate and literal loopback address; never dump environment values |
| `media_worker_database_unavailable` | Both admitted roles, same physical DB proof, migration/post-River readiness and shared startup deadline; no fallback privileged pool |
| `media_worker_start_failed` | Existing fixed `worker_start_diagnostic` phase/category; native River availability and cancellation; never log raw SQL/driver errors |
| `media_worker_stop_failed` | Bounded native shutdown and owned connections; never add provider Stop in the signal path |
| Ready, but attempt remains uncertain | Local readiness is not provider connectivity or health; inspect safe operation events, frozen project/version and missing material keys |

Changing Go TLS/HTTP behavior, LKP validation, LKM format, pgx, River or media
admission requires command unit tests, independent LMW01–05 real-process tests,
the existing LME/LMR gates, and root full PG/race/vet. Keep logs and exact source
revisions. A local test passes only when its actual process, fixture and provider
observations prove the stated outcome; merely emitting ready is insufficient.

LIVE provisioning, merchant HTTP/BFF, studio UI and browser/device testing,
deployment supervision and full G06 remain necessary downstream work.
