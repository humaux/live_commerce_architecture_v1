# T08 LiveKit Egress protocol: LKP01–06

Status: PASS_LOCAL_LKP01_06. This record covers the actual provider-wire
client exercised against isolated local HTTP/TLS fixtures, **not** real LiveKit
Cloud acceptance, broadcast orchestration, G06 or deployment approval.

Contract: [livekit-egress-protocol-v1](../../contracts/livekit-egress-protocol-v1.md).
Dependency/caller notes: [T08 protocol dependency](dependencies.md#t08-livekit-egress-协议依赖).
No module dependency, migration, route, environment credential or customer service
is changed. The integration has no production caller yet.

## Ownership and revisions

Frozen base `921f017`; initial source `237833f`; targeted source repair `98beb86`;
independent tests `0fe0934`. Root integration: `64ca570` + `1f58edc` + `c7cada5`.
The implementation task payload accidentally named `7e5cec8`; the verified base
is **921f017**, corrected before implementation in the assignment and Humaux.

|Role|Model/effort|Worktree and write boundary|
|---|---|---|
|Integrator|inherited root session|main: dependency and acceptance documentation, final tests and integration|
|Protocol author|gpt-6-sol/medium|`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch `commerce/livekit-protocol-go-20260927`; only `internal/integrations/livekit/{client,decode}.go`|
|Independent test author|gpt-6-sol/high|`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch `commerce/livekit-protocol-tests-20260927`; only `internal/integrations/livekit/acceptance_test.go`|
|Independent reviewer|reused read-only agent; inherited model/effort not exposed by its runtime|source, contract, tests and final evidence; no implementation edits|

## Failures retained and root causes

The initial source had three contract defects, independently reproduced before
one targeted source-repair commit:

- An uninitialized, nonnil client let Query/Stop reach a nil HTTP client and
  panic. All three public operations now share a readiness guard returning
  `ErrInvalid` before I/O.
- The JSON depth counter included scalar leaves; a valid 32-container document
  was rejected. The limit now counts opened object/array containers.
- A pagination object with an empty token and an unknown field was rejected.
  Unknown fields are now ignored while invalid/nonempty tokens and duplicate
  aliases still fail before considering empty results.

Complete causal failure evidence:
`/Volumes/data/output/livekit-independent-20260927-source237833f.log`.
Earlier fixture failures remain in `livekit-independent-20260927-initial.log`
and `livekit-independent-20260927-round1.log` under the same output directory:
the former TLS fixture did not trust its own local certificate, and the hanging
handler could block fixture shutdown without draining the request body and an
explicit cleanup release. These are test-harness corrections, not permission to
relax production TLS, protocol limits or test assertions.

## Acceptance stop line

Independent source/test review of the final frozen candidate found no unresolved
P0/P1. The root reran the exact integrated tests; the author was not the sole
validator. The scope is bounded LOCAL provider-double acceptance, not a full
repository or production release gate.

Humaux independent review: `2d1c8703-2e90-41d2-86a5-4b08a0d7a0ed`,
`T08 LKP independent review final 2026-09-27`; implementation record
`487d5ce6-792c-4c3f-8bbf-8ba5817e7571`; independent test record
`ff1e7db3-da46-49d1-9744-fe2de8faeb88`. The reviewer accepted LKP01–05;
the root owns LKP06 documentation and trace closure, not a second security verdict.

|Gate|Independent executable evidence|
|---|---|
|LKP01|Exact Start/Query/Stop method/path/body, JWT independently checked with HMAC and wall clock, both aspect presets and two destinations|
|LKP02|Constructor/input/target bounds, nil and zero clients, typed-nil transport, host slice cloning, redacted formatting/JSON and valid URL boundaries|
|LKP03|Correlation, enums/int64/aliases, duplicate keys, 32/33-container boundary, UTF-8/types/trailing values, gzip decompressed 64KiB pass and +1 reject, pagination checked before empty items|
|LKP04|Real local TLS dropped replies, native request cancellation, separate-origin redirect rejection, HTTP failures; one background-context request hit the 10.00s client ceiling; no repeated calls|
|LKP05|Separate source/test author, independent read-only review, root race/vet plus real PG draft regression|
|LKP06|Dependency/caller notes, retained failures, hashes and Humaux task/canvas/graph trace; no new runtime dependency|

## Commands and evidence

- Independent final `0fe0934`: `go test -race ./internal/integrations/livekit -count=1 -timeout=35s`,
  exit **0**, **12.486s**; `go vet ./internal/integrations/livekit`, exit **0**.
  `/Volumes/data/output/livekit-independent-20260927-race.log`;
  SHA256 `11bb4e15535e5a72210cbd76b9b46bdd735df21c8277396a87c804853826ad8a`.
- Root `c7cada5`: `go test -race -count=1 -timeout=40s -v ./internal/integrations/livekit ./internal/integrations/core ./internal/live`,
  exit **0**: **12 LKP + 4 core top-level PASS, 0 FAIL/SKIP**. LiveKit **12.342s**;
  `internal/live` has no package-local tests, so it is not counted as a test pass.
  `/Volumes/data/output/livekit-root-wire-core-20260927.log`;
  SHA256 `c6a921152641d5638f754a6cd58344f33a6dd579314b1bdea9f3d757a5e1b39e`.
- Root `go vet ./internal/integrations/livekit ./internal/integrations/core ./internal/live`,
  exit **0**; `/Volumes/data/output/livekit-root-vet-20260927.log` (empty success log).
- Root `1f58edc`, same final production source (before adding only LKP tests):
  `bash scripts/dev/test-local.sh --live-planning`, exit **0**, **5 LSP top-level
  PASS, 0 FAIL/SKIP**, real isolated PG18/race, foundation **7.046s**.
  `/Volumes/data/output/livekit-root-live-planning-20260927.log`;
  SHA256 `6dd5934df757dac1ad2de6b76ec8c8ba6844ce6a54f2ac67bfe71def61fee3bd`.
- Causal pre-repair failure log SHA256:
  `46f451cd46bb5309f22dbbb3beea882e5259ffc09cb141ee719f1b9fca1df6cd`.

|Final source/test|SHA256|
|---|---|
|`internal/integrations/livekit/client.go`|`3fd376016cde7046cafa48cfadcb14d7a0cbaa81d9d1c8384b22c30e5c2e1ed9`|
|`internal/integrations/livekit/decode.go`|`0e6bdad7815ec64dc6faf9a45885e3bf38808d0bdac979011807940a29faffef`|
|`internal/integrations/livekit/acceptance_test.go`|`dae8fec4f91cba66103f547534d50b7555bd11f0d6e02841a6f0c124c1b3d762`|

After the PG runner exited, no `livecommerce.fixture` container remained. Test
servers close via their test cleanup; no customer stream or service was stopped.
Committed source, test worktrees and failed/successful logs are retained.

## Remaining integration work

Remaining production responsibilities include tenant/store/attempt entitlement,
protected credentials and stream secrets, durable start/stop and UNKNOWN recovery,
webhook history, revoked-binding stop policy, per-destination projection and
media/billing cleanup. No historical LOCAL intent may become dispatchable by
adding a route. Real Cloud, browser studio and G06 remain NOT_RUN.
