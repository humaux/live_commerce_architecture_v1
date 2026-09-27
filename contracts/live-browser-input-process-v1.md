# Browser input process binding — BRW07 runtime amendment

Status: FROZEN_DESIGN; no runtime activation or real-process acceptance.
Base `73261e3`; prerequisite `live-browser-input-recovery-v1.md`.
Independent review at `4e6220b`: Humaux
`bffee3b8-2ad9-4b53-a2a0-babaeefd6313`, no confirmed P0/P1.

## Existing seams and smallest change

`livekit.LoadWorkerProjects` already runs in parent and exec child; child env
preserves its JSON. Extend that one strict parser, not a second configuration
store. Existing BrowserInputRuntime is only a validated in-memory map.
Input and Egress River workers share `live_media_operation_v1`; registering
both directly would collide. Use one queue-aware worker dispatcher and one
River client, not two independent lifecycle loops or a new job kind.

## Explicit opt-in and exact configuration

Add `COMMERCE_MEDIA_BROWSER_INPUT_ENABLED`: unset/`0` disabled; `1` requests
input in BOTH supervisor and native child; any other value fails configuration.
No enabled input without `COMMERCE_MEDIA_RECOVERY_SUPERVISED=1` and enabled
native worker. Disabled keeps old JSON grammar and old constructor behavior.

With input enabled, each existing COMMERCE_MEDIA_PROJECTS_JSON project may
include exactly one optional `browser_input` object, nonnull when present:

| Field | Validation / purpose |
| --- | --- |
| api_key, api_secret | Explicit input SFU credentials, existing LiveKit validation; never implicitly reuse Egress credentials. |
| mock_dial_address | Existing canonical literal-loopback host:port validator, no DNS/ambient proxy. |
| mock_ca_pem | Existing bounded CA/TLS validation; TLS verifies inherited logical endpoint name. |
| browser_url | Existing canonical ws/wss literal-loopback URL validation. No userinfo/query/fragment/redirect. |

All five fields required, strings only; unknown/duplicate fields rejected.
Retain existing 65536-byte document and 1..128 project caps. At least one
project must have input when enabled. A disabled loader rejects nested input
configuration rather than silently ignoring it. Reject invalid opt-in before
provider I/O. Project ID/version/endpoint/stream-host policy are inherited from
the exact Egress project, not re-specified inside browser_input.

Factor only the existing strict local TLS transport construction for reuse;
same no-proxy fixed-dial policy. Distinct input credentials and transport are
compiled from explicit nested fields. Egress retains its original config and
transport. A real local SFU's HTTP RPC is behind the existing test-harness TLS
proxy; do not weaken TLS verification or add a generic arbitrary-URL transport.
This is an opt-in local MOCK profile, not LIVE/Cloud support.

## Parent and exec child

Both parse the same environment with the same compiled loader and independently
construct BrowserInputRuntime. No parent Go pointer or test-only injection
counts as process binding. Child env keeps the new non-recovery flag and project
JSON while dropping recovery DSN and observer tokens as before.

Parent determines requested scope before loading projects: `1`, or an invalid
nonempty flag, selects mixed diagnostic ledger and unknown coverage on failure.
Retain independent recovery DB/timer even if project/key/worker validation fails.
Never substitute legacy empty for requested-but-unavailable input. Provider
admission requires both constructors/readiness and same physical DB proof.
Child pipe release/EOF/reap, original parent t0 and deadline remain unchanged.

Native construction adds `NewMediaClientWithBrowserInput` preserving legacy
constructor. Validate both maps and old/new worker readiness before ready ACK.
Register one `mediaOperationArgs` worker: exact media_mock_v1 queue routes to
existing mediaExecutionWorker, exact media_input_mock_v1 to existing
browserInputExecutionWorker. Unknown/nil/mismatched job fails closed; original
SQL guards still bind queue/profile/job. Existing 30-second timeout and native
retry/finalization behavior unchanged. Do not invent NextRetry or rescue tuning.
One client owns both queues and jobqueue.Run lifecycle. Existing bounded
concurrency is PER QUEUE (each 1..32); document peak two-queue worker concurrency
as 2N. Observer capacity remains a separate TOTAL N<=32 single wave; no admission
capacity upgrade or throughput claim follows from enabling the input queue.

## Gates before activation

1. Parser/redaction: enabled/disabled/invalid flag; nested unknown/duplicate/null
   keys, remote/ambiguous dial/browser URL, bad CA/keys, missing input; distinct
   transport targets and credentials, no formatter leakage or provider I/O.
2. Compiled parent+exec child both reconstruct exact fixture mapping. One client
   actually consumes both queues; no duplicate-kind registration, cross-routing
   or lost original job. Legacy-only path stays unchanged.
3. Actual unaged SIGKILL/relaunch + real PG + local SFU and mock Egress: one
   episode/t0 and exact composite readback or sticky unresolved timeout/alert.
   Invalid input config retains diagnostic scope; child cannot run input early.
4. Existing BRW and MRR gates, native CLI, role/physical-DB negatives and full
   regression. External alert delivery and Cloud/G06 remain separate release
   requirements. No customer configuration or production enabling in this task.
