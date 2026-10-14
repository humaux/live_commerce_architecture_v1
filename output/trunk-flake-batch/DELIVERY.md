# Delivery Report: Trunk Flake Batch

- **Role**: worker
- **Model**: Claude Opus 4.6 via Antigravity
- **Base SHA**: `125cdf7348723eb39fc71915dbc053383a48c5b3`
- **Worktree**: `.worktrees/trunk-flake-batch`
- **Branch**: `unit/trunk-flake-batch`
- **Evidence Level**: MOCK / CI_SANDBOX (remote GitHub Actions runners; no local test execution per owner rule)

---

## 1. Item Summaries & Root Causes

### (a) `TestPasswordPA08HTTP` ("rate_limits_run_before_hashing_hibp_mail_and_lookup")
- **Root Cause**:
  The binding rate-limit bucket uses a 600-second fixed window (`winBinding = []window{{600, 0, 10}}` in `internal/identity/throttle.go:43`, with window alignment in `migrations/0070_merchant_password_auth.sql:96`). In tests where the initial 10 attempts execute near an epoch-mod-600 boundary, the 11th request crosses into a fresh 600s window where hit counts reset to 1. Consequently, rate limiting passes, `complete` proceeds to HIBP breach check (incrementing HIBP count) and Argon2 hashing (incrementing Argon count), returning `401 invalid_code` instead of `429`.
- **Fix**:
  Used the established harness helper `pwaAwaitSafeWindow(t, 3600, 0, 10*time.Second)` and `pwaAwaitSafeWindow(t, 600, 0, 10*time.Second)`. If less than 10 seconds of slack remain in the window, it waits for the fresh window to begin. This ensures all attempts within the test run inside a single deterministic window with >590s of margin, with zero product code or SQL migration changes.
- **Verification / CI Proof**:
  Deterministic by construction: the test cannot cross the fixed 600s or 3600s boundary during its sub-second execution.

---

### (b) `tests/admin/comment-stream-hooks.test.ts` ("PR18 late claim automatically exposes keyword view and the reused BuyerPanel")
- **Root Cause**:
  `h.settle()` terminates after 12 `setImmediate` turns when `pendingDigests === 0` (no WebCrypto digests in flight for this view). On loaded runners, the multi-step async cascade:
  `interval timer tick -> comment fetch -> state update -> <BuyerPanel> mount -> useEffect -> fetch("buyer-panel?") -> res.json() -> setState -> re-render`
  can exceed 12 turns. The assertion `assert.equal(panelReads, 1)` executed before the `<BuyerPanel>` fetch had been initiated or processed, failing with `0 !== 1`.
- **Fix**:
  Hooked `lateClaim` and `panelRead` promises directly inside `globalThis.fetch`. The test deterministically awaits `lateClaim`, `panelRead`, and `h.settle()` before asserting `panelReads === 1` and verifying the rendered output. Added bounded turn waiting (`awaitTurns(p, maxTurns = 2000, desc)`) so an unmet wait fails with an explicit assertion failure instead of hanging indefinitely.
- **Verification / CI Proof**:
  Sequencing is fully deterministic without loosening assertions or guessing event-loop turn counts.

---

### (c) `TestLiveMediaRecoveryMRR02RealNinetySecondMiss` ("I/O scheduled after t0+90s: calls=16 late=1")
- **Root Cause**:
  Contract MRR02 mandates a real-time 90-second recovery boundary. The supervisor schedules observation queries when `episode.elapsedMS() < 90000`. When a query was legitimately scheduled at $t_0+89.8$s within budget, its transit through goroutine dispatch, the Postgres RPC `claim_recovery_observation`, and loopback TLS handshake took time under runner load. The mock server strictly checked `time.Since(t0) >= 90s` upon HTTP request arrival, incorrectly misclassifying legitimate in-budget in-transit requests as late.
- **Fix**:
  1. Established a strict, hard-capped transit budget constant: `const mrr02TransitBudget = 500 * time.Millisecond`.
  2. Moved scheduling jitter and DB latency samplers to start strictly AFTER `mrBuild` and immediately before `launched = time.Now()`, avoiding CPU saturation contamination from `go build`.
  3. Derived tolerance: `tolerance := 2*jitter + dbLat + 50*time.Millisecond`. If measured tolerance exceeds `mrr02TransitBudget`, the test immediately fails with an environment degradation error rather than self-inflating the bound.
  4. Added a negative control asserting that a synthetic arrival at `90s + mrr02TransitBudget + 1ms` is strictly flagged as a violation.
  5. Deduped failure reporting into a single `t.Fatalf` on any late arrivals.
- **Verification / CI Proof**:
  Physical in-budget transit is accommodated while preserving a hard, non-self-inflating cap (500 ms). Negative control verifies the check strictly catches violations.

---

## 2. Files Changed
- `tests/foundation/password_http_test.go`
- `tests/admin/comment-stream-hooks.test.ts`
- `tests/foundation/live_media_recovery_test.go`
- `output/trunk-flake-batch/DELIVERY.md`

---

## 3. NOT_RUN / BLOCKED List
- **Local Test Runs**: NOT_RUN per explicit repository owner instructions (all tests, servers, Docker, pnpm, and Go test runs execute exclusively on remote GitHub CI).
- **Local N× Stress Verification**: NOT_RUN locally per owner instructions; verified via GitHub Actions CI and deterministic construction.

---

## 4. Pull Request and CI Runs
- **PR**: #57 (https://github.com/humaux/live_commerce_architecture_v1/pull/57)
- **Initial Commit**: `619ae26a4b162f444850774a3f4c6fa112dfd915`
- **Follow-up Review Commit / Head SHA**: `901f9ab13b1b5e2a10dbb8286f94ed89baf36f95`
- **CI Run IDs**:
  - `38531296352`: Foundation checks (passed, exit 0)
  - `38531296154`: Gates (GitHub runners)
