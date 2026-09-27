# Browser input configuration acceptance — 2026-09-27

Status: **CONFIG_ONLY_LOCAL_UNIT_ACCEPTED**. Root tested tree `a389f9b`.
Full T08/T09/SaaS and product BRW runtime remain in progress. No deployment.

## Scope and independent workflow

- Design: [BRW](../../contracts/live-browser-input-worker-v1.md), base `5a543e7`,
  freeze `f518c77`; independent bounded review
  `7ca5ab59-6fc3-4d40-ab06-31bf01c9dfed`. The earlier `d7a30f3` review separately
  allowed only the no-activation constructor while two route/claim clarifications
  were pending; these and the HTTPS fixture obligation are now addressed.
- Source: `a3d5f8f` plus repair `d5fc84f`, integration_worker
  `browser_input_config_impl`, requested gpt-6-luna/high, base `3bdf60a`,
  existing isolated worktree `commerce-meta-inbox-go-20260926`; only
  `internal/live/browser_input_runtime.go` (107 lines after repair).
- Independent tests: `6ac615b`, test_worker `browser_input_config_tests`,
  requested gpt-6-sol/medium, same base, separate existing worktree
  `commerce-meta-inbox-tests-20260926`; only the 209-line matching test file.
  Old untracked `output/` evidence was retained. Neither agent recursively
  delegated or changed dependencies, migrations or production configuration.
- Root reviewed both diffs, integrated tests in an isolated tree, then merged
  the exact passing source/test contents as `a389f9b`. No existing executable
  file changed. Actual provider calls, token signing and route/queue activation
  are absent. Grep finds constructor callers only in the independent test file.

The constructor validates 1..128 version-keyed MOCK projects, canonical loopback
browser URLs, existing LKI config/transport validation and constant redaction.
It reuses `mediaProjectKey`, `ErrMediaConfig`, `livekit.New` and the standard
library; no generic provider abstraction or new dependency. A trusted injected
RoundTripper is NOT proof of network routing or a production environment.

## First failure preserved; root-cause repair

Root's first combined tree `c175304` failed the independent race test:
3 top-level PASS / 1 FAIL. Both accepted IPv6 examples failed because the source
assembled `[::1:port]`, not `[::1]:port`. Root identified the same defect by diff
review. Targeted repair 1 replaced manual brackets with `net.JoinHostPort`;
no test, threshold or rejection check was weakened.

- Failure log: `/Volumes/data/output/brw-config-independent-first-20260927.log`
- SHA-256: `036f0f488e9c1b68d8c25f013aa4651459d20e72d165f09ea2f18fbb3290799c`
- Repair memory: `3161c889-f996-4a02-9c2a-a11e533a9809`, linked to
  `canonicalBrowserInputURL` in the code graph.

## Executed acceptance

Root ran these commands, not merely the source author's existing tests:

```sh
GOTOOLCHAIN=go1.27.1 go test -race -count=1 -run '^TestBrowserInputRuntime' -v ./internal/live
GOTOOLCHAIN=go1.27.1 go test -race -count=1 -v ./internal/live ./internal/integrations/livekit ./internal/httpapi ./cmd/media-worker ./cmd/api
GOTOOLCHAIN=go1.27.1 go vet ./internal/live ./internal/integrations/livekit ./internal/httpapi ./cmd/media-worker ./cmd/api
```

|Gate|Actual result|Evidence|
|---|---|---|
|Isolated repaired candidate `14dca99`|Exit 0, 4 top-level PASS, 0 FAIL/SKIP, race; 2.267s|`/Volumes/data/output/brw-config-independent-repair1-20260927.log`|
|Root `a389f9b` five packages|Exit 0, 79 top-level PASS, 0 FAIL/SKIP; race and following vet passed|`/Volumes/data/output/brw-config-root-regression-20260927.log`|
|Content equality|No source/test diff between `14dca99` and `a389f9b`|Root Git comparison before regression|

Hashes, respectively:

- `ae00d15bdc884e3fe6b1c435f5ac8bb1916e167684a0a1cd067ac2de83021a74`
- `0e5f725e42ba7f2cf8ed5525d2bc8171e924339734161cea600b2abbc90cc878`

Tests cover cardinality and duplicate version keys, invalid/typed-nil config,
strict IPv4/IPv6 URL grammar and remote/encoded/credential/port variants,
nil-on-error/no partial result, redacted formatting/JSON and zero RoundTrip
calls on success or rejection. The two changed code files were indexed and
linked to their ABI/fix decisions.

No PG fixture or browser/SFU was started for this configuration slice. Go
commands exited; failure and success logs remain available. No cleanup of old
artifacts, customer resources or shared caches was performed.

## Explicitly not accepted by these tests

BRW SQL marker/claims/wire reservations/joint native guard, the input-queue
consumer, bounded cleanup and original-job recovery, post-commit HTTP token
delivery, the HTTPS product browser/SFU receiver gate, new input controls and
Cloud revocation are still pending. The earlier full PG 696 PASS receipt is
historical baseline evidence, not a fresh full run on `a389f9b`.

Next implementation consumes the frozen ABI, with root-owned forward migrations
and separate source/test worktrees. Preserve original operation/job, zero
provider calls for marker-0/UNISSUED cases, before/after-wire deadline separation,
and independent input/Egress liability. Re-run actual PG/worker/browser gates
before activating any token-delivery path. Customer production is untouched.
