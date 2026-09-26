# T08 LKP07: lost-Start room discovery acceptance

Status: **PASS_LOCAL_LKP07**. Root source/test revision `452fd66`; this adds
room-scoped observation recovery to the previously accepted LKP01–06 client.
No production caller, dispatcher route, credential configuration, migration,
customer session or external provider was changed. Durable controller recovery,
real Cloud/media, browser lifecycle and G06 remain **NOT_RUN**.

## Cause, smallest repair and trust boundary

`Query(Target)` requires the Egress ID. A provider may accept Start and lose its
response before this ID is known, so that API cannot discover the resource.
`FindByRoom` makes one ListEgress request for the persisted server-generated room
and returns exactly one valid same-room candidate. It reuses the bounded transport
and strict decoder; no SDK, queue or new dependency. `Query` continues to require
and correlate an explicit ID. Empty or ambiguous discovery never authorizes a
second Start, Stop, ownership adoption or a terminal state.

Contract [LKP07](../../contracts/livekit-egress-protocol-v1.md) frozen at `40335e2`
after independent preflight `4048fee1-d4d2-4316-a653-c25b94d37f20`. The caller must
load the room from a trusted persisted attempt, not merely validate user syntax.
Future controller responsibilities and NOT_RUN gates are in the separately
reviewed [media authority decision](2026-09-27-media-authority-lifetime.md).

## Ownership and revisions

|Role|Actual model/effort|Base, head and write scope|
|---|---|---|
|Source author|gpt-6-sol / medium|base `40335e2`, head `0d34d1a`; `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch `commerce/livekit-room-go-20260927`; only `internal/integrations/livekit/{client,decode}.go`|
|Independent test author|gpt-6-sol / high|base `40335e2`, source cherry-pick `f0f43e1`, test-only head `e9da41c`; `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch `commerce/livekit-room-tests-20260927`; only new `internal/integrations/livekit/room_discovery_test.go`|
|Independent reviewer|Inherited model/effort not exposed by runtime|Read-only contract/source/tests and root evidence; no implementation edits|
|Root integrator|Inherited root session|main `e99f77b` source + `452fd66` tests; independent replay and documentation|

Source author experienced a model-capacity error after the source commit. A
follow-up completed its task/memory/canvas/graph handoff without changing source.
Its earlier successful race/vet outputs exist only in its tool transcript, not a
standalone log; acceptance below relies on retained independent and root logs.

## Executed gates

- Local TLS server accepts Start, retains the resource condition and closes the
  connection before responding. Client reports ErrUnknown, then room discovery
  returns the candidate: **Start 1 / List 1 / Stop 0**. Exact method/path/body/host
  and HS256 JWT are independently checked, including absence of `egress_id`.
- Invalid room, nil context/client and zero client fail before I/O; pre-cancelled
  request never reaches the server. Existing Query still requires and matches ID.
- Empty/omitted list remains NotObserved; wrong room/ID, duplicate/multiple rows,
  pagination before empty, alias/type/UTF-8/size/malformed JSON fail conservatively.
- HTTP errors, redirect target and native cancellation are tested against local
  TLS endpoints, with static errors, redaction and exact request counts.
- Full existing LKP tests remain, including the real 10-second timeout and strict
  decoder boundaries. No failure was bypassed and no LKP07 source repair needed.

|Executor and command|Exit and result|Retained log|
|---|---|---|
|Independent `go test -race ./internal/integrations/livekit -run '^TestLKP07' -count=1 -timeout=30s`|0|`/Volumes/data/output/livekit-room-independent-20260927-initial.log`|
|Independent `go test -race ./internal/integrations/livekit -count=1 -timeout=45s`|0; 11.964s|`/Volumes/data/output/livekit-room-independent-20260927-race.log`|
|Independent `go vet ./internal/integrations/livekit`|0|`/Volumes/data/output/livekit-room-independent-20260927-vet.log`|
|Root `go test -race -count=1 -timeout=45s -v ./internal/integrations/livekit ./internal/integrations/core ./internal/live`|0; **18 LKP + 4 core top-level PASS, 0 FAIL/SKIP**; 12.236s / 1.876s|`/Volumes/data/output/livekit-room-root-race-20260927.log`|
|Root `go vet ./internal/integrations/livekit ./internal/integrations/core ./internal/live`|0; empty success log|`/Volumes/data/output/livekit-room-root-vet-20260927.log`|

`internal/live` has no package-local tests and is not counted as a pass. No database
code or caller changed; the earlier PG draft/full-regression records were not
rerun here and are not represented as current LKP07 evidence.

|Evidence/source|SHA256|
|---|---|
|Root race log|`f4024f68ed18dce7368effa8865d3f35bf6915cd178e06781eba9a095f831c25`|
|Independent race log|`cb1d24d640712f4b4d3b5c414978fd756956e8c95270c1c6337931a66ec38819`|
|Independent targeted log|`a0a89405d790e86b78748b27ac68a6cad9436123039cfc67b50875f690b4168b`|
|client.go|`0e24c89132a4651925699048c9bf714a9f69874bf5bdda3cce7092d512688487`|
|decode.go|`1464ba4761720e0454d882075234499beb0b78946c470a7e07fd7ff14a9591ea`|
|room_discovery_test.go|`2ca5f038e51e5a04577ef6d859e1948d3afe7b9502058801bc7ecf3694bd9f70`|

Humaux source receipt `15bf0a01-77d3-47b8-8026-7703015d95e7`; independent test
receipt `87641159-f1d0-49a6-8456-08dd2186d940`; reviewed media DESIGN decision
`c2e13f28-fa6a-4f33-8019-82905c2480d9`; independent final source/test/design review
`add5a91b-f012-41db-8eca-c4970952fd78` found no open scoped P0/P1. Local TLS fixtures close with tests; no
container was started for this slice. Worktrees and evidence are retained.
