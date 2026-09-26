# T08 LKM01–05: sealed media material acceptance

Status: **PASS_LOCAL_LKM** for the custody component at root `e4b704a`.
No database, credential configuration, provider caller, migration, customer
session or production service changed. Durable controller/MLA, real Cloud/media,
browser lifecycle, G06 and deployment remain **NOT_RUN**.

## Cause, implementation and dependency boundary

The durable controller needs destination stream keys without putting plaintext
in ordinary operation/job/audit JSON. `livekit.MaterialKeyring` provides that
custody boundary using standard-library AES-256-GCM, fresh 12-byte nonces and a
defensively copied keyring. It reuses the existing client validators and strict
JSON decoder, not PAYUNi credential types, keys or database permissions. No new
module dependency, SDK, vault, environment reader or network call was introduced.

The frozen [LKM contract](../../contracts/livekit-material-v1.md) at `fd4e674`
defines the exact format-1 AAD field order. It binds tenant/store/session/attempt,
project, credential/material versions, environment, endpoint and key ID. The
private plaintext has only room, aspect and stream URLs. Public formatting and
JSON remain redacted; persistence must use explicit envelope fields. Both Seal
and Open validate the derived room and current client allowlist. Every failure
returns the static ErrMaterial with zero output.

This primitive grants **no authority to start or stop**. The future SQL resolver
must load trusted scope/project versions and revalidate the lease and permission.
Ciphertext authenticity is not destination ownership or current live eligibility.

## Ownership and revisions

|Role|Actual model/effort|Base, head and write scope|
|---|---|---|
|Source author|gpt-6-sol / medium|base `fd4e674`, source `8692251`, comment-only `c0ab5ed`; `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`, branch `commerce/livekit-material-go-20260927`; only `internal/integrations/livekit/material.go`|
|Independent test author|gpt-6-sol / high|base `fd4e674`, source cherry-picks `363cfe5` and `435df19`, test-only `3614d39`; `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch `commerce/livekit-material-tests-20260927`; only `internal/integrations/livekit/material_test.go`|
|Independent reviewer|Inherited model/effort not exposed by runtime|Read-only contract/source/tests and root evidence; no source/test changes|
|Root integrator|Inherited root session|main source `a18edd5`, comments `efd9109`, tests `e4b704a`; independent replay and documentation|

## Executed gates

- LKM01: independent ordered JSON/AAD builder and standard-library AES-GCM
  interoperate in both directions; both aspect ratios and two outputs round-trip;
  successive seals yield distinct nonces/ciphertexts and zero provider calls.
- LKM02: every scope/version/context field, same-byte key-ID alias, nonce/tag/
  ciphertext, wrong key and retired key are rejected. Retained old keys open old
  envelopes; new envelopes use the new active key. Endpoint trailing slash is
  canonically equivalent, not a new identity.
- LKM03: independently authenticated malformed/duplicate/unknown/missing/aliased/
  wrong-type/invalid-UTF8/trailing JSON, wrong room/aspect/URL, invalid scope/input
  and nonce/ciphertext bounds fail. The 16401-byte fixture is valid AES-GCM, not
  merely random invalid ciphertext; source confirms the cap precedes decryption.
- LKM04: nil/zero objects, invalid keyrings, external map/key mutation, returned
  slice ownership, concurrent use and redaction canaries. Exact ErrMaterial and
  zero output checked. No new failure or source repair was needed.
- LKM05: independent tests, root race/vet replay and prior LKP/core regressions.
  Seal/Open cannot dispatch a provider request; that is not a controller gate.

|Executor and command|Exit/result|Retained log under `/Volumes/data/output/`|
|---|---|---|
|Author `go test -race -count=1 ./internal/integrations/livekit`|0; 12.639s, existing LKP tests only|`livekit-material-author-20260927-race.log`|
|Author `go vet ./internal/integrations/livekit`|0|`livekit-material-author-20260927-vet.log`|
|Independent `go test ./internal/integrations/livekit -run '^TestLKM' -count=1 -timeout=20s`|0; 0.643s|`livekit-material-independent-20260927-initial.log`|
|Independent `go test -race ./internal/integrations/livekit -count=1 -timeout=45s`|0; 12.228s|`livekit-material-independent-20260927-race.log`|
|Independent `go vet ./internal/integrations/livekit`|0|`livekit-material-independent-20260927-vet.log`|
|Root `go test -race -count=1 -timeout=45s -v ./internal/integrations/livekit ./internal/integrations/core ./internal/live`|0; **7 LKM + 18 LKP + 4 core top-level PASS, 0 FAIL/SKIP**; 13.054s / 2.244s|`livekit-material-root-race-20260927.log`|
|Root `go vet ./internal/integrations/livekit ./internal/integrations/core ./internal/live`|0; empty log|`livekit-material-root-vet-20260927.log`|

`internal/live` has no package-local tests and is not counted as PASS. Existing
PG integration tests were not rerun because no database/caller code changed;
prior PG results are not promoted to new controller acceptance.

|Evidence/source|SHA256|
|---|---|
|Root race log|`c19365a47a548450637bb288583f187ca1944b23a3d8a8c72aae65fe93396187`|
|Independent race log|`4b2927afab7a381dd85ed0adbca9b7f7025e08eb03184d9a8323338113812e99`|
|material.go|`2a5479984ce9acb22709475c50df34b620b0948f83cdf89be7b14c891289b6e0`|
|material_test.go|`27961f5db88f816fae00b7e203a07f5f34dd28c41bb2f845b0c9b0edd596cc34`|

Humaux author receipt `753f407e-4f59-4068-a143-c461ebeb3918`; independent tests
`ba6322fd-5b35-4c96-a8a6-9cee130684cc`; final independent local acceptance
`b08b3d1a-9077-485f-bf0e-84631610fcf4` verified the root log hash/counts and found
no scoped P0/P1. Controller freeze-blocker correction is separately recorded as
`c81b5904-74c5-4c24-8cbc-fd87f65ec450`, not LKM acceptance. Local fixtures close with tests. No container
was started; worktrees and evidence are retained. The separately reviewed
[controller candidate](../../contracts/live-media-controller-v1.md) is NOT FROZEN:
exact SQL roles/FKs/lease/terminal mapping and lost-before-delivery Stop recovery
still need contract closure before implementation and real-role fault tests.
