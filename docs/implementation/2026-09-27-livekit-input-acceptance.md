# LKI browser-input protocol acceptance — 2026-09-27

Status: **PASS_LOCAL_PROTOCOL_ONLY**. Independent tests and Root regression on
`1db26fe` passed; fixed-source review found no confirmed P0/P1. No Cloud,
production or customer resource was used. Product BRI01–07, real Egress output
and global G06 remain open.

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
|LKI01 publisher claims/redaction|PASS_LOCAL_PROTOCOL|Independent exact HMAC/claims, deterministic token, rejection before I/O, formatting/JSON redaction|
|LKI02 exact TLS wire/grants|PASS_LOCAL_TLS_DOUBLE|Four exact RPC paths/bodies/least management grants; invalid target/cutoff before I/O|
|LKI03 strict decoding/correlation|PASS_LOCAL_TLS_DOUBLE|Identity/room correlation, enums/defaults/mute/source/type, duplicate/array/UTF8/size/gzip errors, ACK-only response handling|
|LKI04 faults/no retries|PASS_LOCAL_TLS_DOUBLE|Real local drop/hang/cancellation and redirect/404/503/malformed200; one call and no redirected recipient|
|LKI05 independent review/root regression|PASS_BOUNDED_LOCAL|Independent fixed-source review; Root41 race tests, isolated PG18/media process8, full-repo vet; see exact scope below|

### Immutable source and independent evidence

Author `223d4bf83e1f1f489b3780c1cad881e0c799e16c`, two source files only; main
cherry-pick `7c40be6`. Independent test-only `16357fb0bf3109058b0ec556b2fd4e744d12feb7`
on source cherry-pick `1dcfc91`; main test cherry-pick `c0deb41`. Root `1db26fe`
adds only explanatory code comments. It is the exact revision independently run
by Root below. Existing Egress tests were not changed or weakened.

Independent `go test ./internal/integrations/livekit -run '^TestLKI' -count=1 -v`
passed all6 top-level new tests. Full package `go test -race -count=1` and package
`go vet` both exited0. Log:
`/Volumes/data/output/lki-independent-race-16357fb-20260927.log`, SHA256
`2aa00045eece665e38385f7714500a21f80e812f4710a65ce61d6af1b8b1f915`.
Humaux independent receipt: `4dd681ea-ed85-4f24-922c-2aa68448f225`.

Read-only independent review of fixed author `223d4bf` vs `0f9babe`: no confirmed
P0/P1; no test/provider execution by that reviewer. Receipt
`bde7735a-ee51-4a53-8cc8-78c6b959b9e1`. Root also read the full changed source,
private Egress-helper diff and all independent tests before integration.

### Root commands and actual outcomes at 1db26fe

1. `go test -race -count=1 -timeout=90s -json ./internal/integrations/livekit ./internal/integrations/core ./internal/live ./cmd/media-worker`
   — exit0, **41 top-level PASS / 0 FAIL / 0 SKIP**, including6 LKI tests;
   LiveKit package13.679s. Log
   `/Volumes/data/output/lki-root-1db26fe-tests-20260927.jsonl`, SHA256
   `02d07a967cc914a919d76aa525b3ebc694438274dfb03d09659e9dc5b7e812c1`.
2. `bash scripts/dev/test-local.sh --live-media-runtime`
   — exit0, **8 top-level PASS / 0 FAIL / 0 SKIP**, foundation26.145s. Actual
   temporary PG18 and media-process startup/stop/restart/missing-config recovery,
   local TLS provider only. Log
   `/Volumes/data/output/lki-root-1db26fe-runtime-20260927.log`, SHA256
   `f3f7553f70508feb28bff309c4507db033f34647a091f734b482a406d71564e3`.
3. `go vet ./...` — exit0. Empty output file
   `/Volumes/data/output/lki-root-1db26fe-vet-20260927.log`, SHA256
   `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

The two test commands have separate scopes, not49 claimed unique tests. This is
not a fresh all-foundation or browser run. No failing execution was observed in
this wire increment; earlier probe failures remain in their original evidence.
`git diff --check` passed. The temporary fixture script exited normally and a
postflight `docker ps -a --filter name=lc-foundation-test` returned no containers.
Local TLS listeners are test-owned and closed with the test process; no external
API or customer broadcast was invoked. Evidence logs and reusable worktrees stay.

After comment-only integration, three changed Go files were indexed in Humaux:
done,3 files/99 entities/0 rejected. This protocol pass does not establish token
acceptance on a real SFU or actual Cloud revocation; a TLS double checks wire only.

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
