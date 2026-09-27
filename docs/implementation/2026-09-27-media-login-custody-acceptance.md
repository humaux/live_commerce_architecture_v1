# Media login custody acceptance — 2026-09-27

Status: **IMPLEMENTATION_IN_PROGRESS; MLC01–06 NOT_RUN**.
Frozen contract: [MLC v1](../../contracts/live-media-login-custody-v1.md),
`5db67f5`; independent contract review at `3cbdaea` found no remaining confirmed
P0/P1 after fixing replay's missing revision GUC. Source acceptance is separate.

## Scope and ownership

This binds the existing MOCK media Start to its exact initiating merchant login,
so pre-wire logout prevents Start and post-wire logout retains cleanup authority.
It does not issue browser credentials, grant LIVE access, or implement input
cleanup after Egress terminal. BRI01–07 and overall T08/G06 remain unaccepted.

|Role|Worktree/branch|Allowed writes|
|---|---|---|
|source `/root/media_runtime_source`|`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`; `commerce/media-login-source-20260927`|`migrations/0039_live_media_login_custody.sql`, `internal/live/media_plan.go`|
|independent tests `/root/media_runtime_tests`|`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`; `commerce/media-login-tests-20260927`|`tests/foundation/live_media_login_custody_test.go`|
|fixed-source reviewer `/root/livekit_protocol_impl`|Read-only main/candidate commits|None|
|integrator `/root`|Main checkout|Final migration merge, docs, independent regression|

Both writers start at `5db67f5`. Existing idle agents/worktrees are reused;
their current runtime model/effort is not exposed by the tools. No new override
or recursive delegation. Maximum two simultaneous source/test writers.

## Required evidence

Run independent MLC cases against actual isolated PG18/roles. Prefix test names
`TestLiveMediaExecutionMLC` so the existing `--live-media-stop` selector includes
them, avoiding another test runner. Also replay native media-process and Studio
tests for source/replay compatibility. Root must independently rerun fixed code.

Do not replace actual login IDs with only a principal, label a fabricated old
row as an upgraded real identity, or weaken pre-existing queue/authority gates.
Existing permission grants do not automatically increment authz revision;
transient revoke-and-regrant without such an increment is an explicit remaining
permission-management prerequisite, not an accepted revocation guarantee.

Source/test SHAs, measured results, failures, cleanup and accepted scope will be
filled only from executed evidence. No customer service or provider write is
authorized by this record.
