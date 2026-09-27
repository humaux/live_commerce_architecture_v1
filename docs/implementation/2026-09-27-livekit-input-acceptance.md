# LKI browser-input protocol acceptance — 2026-09-27

Status: **IMPLEMENTING_NOT_ACCEPTED**. All new LKI executable gates are NOT_RUN
until their actual results are entered below. No Cloud, production or customer
resource is used. Product BRI01–07, real Egress output and global G06 remain open.

## Scope and review baseline

- Root base: `0f9babe`, frozen [LKI01–05 wire contract](../../contracts/livekit-input-protocol-v1.md).
- [Product lifetime boundary](../../contracts/live-browser-input-v1.md) reviewed
  independently at `3d72d58`, Humaux `c74c83b8-72be-4405-9e54-016869df3a30`.
- Wire preflight at `84280e9`: no confirmed P0/P1, Humaux title “LiveKit input
  wire LKI01–05 independent preflight”; async ID not returned at dispatch.
- Design fixes retained in history: `89ae0db`/`52f6a6c` distinguish revoke before
  vs after final dispatch gate; `52f6a6c` requires actual decoded receiver media
  and original-job cleanup through Egress terminal; `3d72d58` separates local
  input/Start sequencing from real Cloud Egress output.

Only a publisher JWT signer and four fixed RoomService methods are in this code
slice. They reuse the existing trusted Client, HTTP limits, strict parser and
HS256 primitives. No merchant route, database authority, credentials, new SDK,
queue, migration or deployment is introduced.

## Isolation and ownership

|Role|Base/worktree|Allowed paths|
|---|---|---|
|integration_worker `/root/media_runtime_source`|`0f9babe`; `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch `commerce/livekit-input-wire-20260927`|new `internal/integrations/livekit/input.go`, minimal private helper changes in `client.go`|
|test_worker `/root/media_runtime_tests`|`0f9babe`; `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch `commerce/livekit-input-tests-20260927`|new input-prefixed `_test.go` only|
|integrator `/root`|`/Volumes/data/live_commerce_architecture_v1`, main|reviewed cherry-picks, contract/task/dependency/acceptance documentation; independent rerun|

Prior inherited source/test model labels were gpt-6-sol/medium and gpt-6-sol/high;
the current agent runtimes do not expose independently verifiable model/effort.
No new override or recursive delegation was used. At most two writers.

## Results

|Gate|Status|Evidence|
|---|---|---|
|LKI01 publisher claims/redaction|NOT_RUN|Implementation/test work pending|
|LKI02 exact TLS wire/grants|NOT_RUN|Implementation/test work pending|
|LKI03 strict decoding/correlation|NOT_RUN|Implementation/test work pending|
|LKI04 faults/no retries|NOT_RUN|Implementation/test work pending|
|LKI05 independent review/root regression|NOT_RUN|No final source SHA yet|

Source SHA, independent test SHA, commands, exit codes, counts, failure evidence,
root rerun and transient-resource cleanup must be filled from actual runs before
this file can say accepted. A test double verifies protocol, not Cloud guarantees.

## Required downstream work

Product admission must persist scope and identity before returning a token,
recheck live merchant/login authority, and keep the original durable media job
responsible for input cleanup even after Egress is terminal. Initial token TTL,
successful Remove/Delete ACK, and one empty ListRooms response do not prove that
an SDK-refreshed token cannot rejoin. The Cloud revocation profile must pass its
separate cached-token/clock/missing-room gates before LIVE admission. Full-system
outages retain an overdue/UNKNOWN resource obligation, not an invented hard cap.

Studio visual selection remains independent of the approved merchant orders C
layout. No Studio UI code or device permission prompt is part of this slice.
