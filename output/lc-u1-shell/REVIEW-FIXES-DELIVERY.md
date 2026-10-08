<!-- Purpose: Bind the LC-U1 independent-review repairs, scope audit and local evidence to source 1fe069cc; request new GitHub acceptance.
Depends on: live-console-v1 cadence, owner UNKNOWN/403 ruling, trunk0813424a, real LC-B7 A1 and prior CI37597312124.
Used by: Integrator push/CI/review; earlier green CI is not acceptance of changed source. -->
# LC-U1 — independent review repairs

- Branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Source: **`1fe069ccc5c90670fb1f617897af2576ee87a1a4`**; merge `ae9493bc` includes `origin/r3/integration` **0813424a**. Final evidence-only commit preserves source hashes below.
- Parent model not exposed by runtime. Two read-only explorers used inherited model with explicit medium/high reasoning; no recursive delegation or parallel code writers.
- Prior CI verified with `gh run view 37597312124`: completed/success, **34/34**, head **70b9f4e0fa1364578568e8c78a3ae35f6deb1131**. It predates these fixes.

## Findings closed

1. **P1 backoff:** Studio errors carry HTTP status and Retry-After seconds/date. A1 polls five seconds after success; consecutive failures back off **3/6/12/24/30 s**, respecting longer server delays. Success resets the counter. Hidden tabs abort/stop; 401/403 conceals data and latches until a scope remount. Long delays are split into safe JS timer intervals.
2. **P2 reads:** only A1 opts into periodic `useLiveRead`. Session list/settings/results default to mount/explicit refresh. Console detail/source/offer controls are independent reads on entry/refresh/command; a failed secondary cannot erase A1 statistics and cannot authorize stale controls. Existing Studio media-attempt reconciliation is unchanged (separate pre-LC-U1 recovery behavior).
3. **P2 journal:** sessionStorage stores validated **key/method/path/body** plus ambiguity flag, never CSRF/session credentials. Same-scope explicit reload retry uses original bytes/key and existing receipt parsers. Success clears only its matching key even after departure; stale callbacks/navigation are suppressed. Restored copy completion navigates to the new scene or conflict settings.
4. **Owner safety ruling:** a first definitive 4xx, including 403, clears its journal and permissions lock the UI. **Prior UNKNOWN/in-flight/reloaded request → later 403 stays fenced**: that refusal cannot settle the earlier attempt. Ambiguity is durable before dispatch, not after an ACK, so reload/departure cannot erase it. Legacy key-only/corrupt journals remain fail-closed, without guessed replay.
5. **P2 recommendation:** BFF and journal reject `post_comment:true`. Only `false` is emitted. No external Page comment path is enabled.
6. **P2 copy/notices:** frozen host prompt block restored byte-for-byte to trunk (hash pinned); Japanese text/picker widening removed. Existing three-mode Go/SQL parser/picker remains. The frozen non-EXACT wording is deliberately reused for CONTAINS per ruling, rather than retaining new UI-authored text. IG notice only for Instagram source or `video_embeddable:false`. Stale LC-B7/embed comments corrected. Recovery no longer sends the user to an unspecified administrator.
7. **P2 evidence:** fake-timer hook execution covers hidden pause/403 lock/replay; actual component SSR proves A1-only cadence selection, secondary degradation and three-locale notice/denial states. A new focused PG test forwards **raw JSON returned by the real LC-B7 merchant A1 route unchanged** to production `parseConsole`, not a handwritten DTO.

## Scope audit against origin/r3/integration

| Earlier shared-path delta | Disposition / reason |
| --- | --- |
| `claims-model.ts` | KEEP enum parity: Go/SQL already supports `KEYWORD_QTY_CONTAINS`; rejecting it breaks settings. No new server mode. |
| `claims-copy.ts` | REVERT unsupported Japanese/new host text; exact frozen block restored. Three-locale selector aliases remain outside that block. |
| `proxy.ts` / catch-all BFF | KEEP exact Console/Results route grammar and validated shapes, not a generic proxy or permission relaxation. Merge retains trunk payment/media routes. |
| `next.config.ts` | KEEP named CSP constant with **byte-identical policy**, no `frame-src` or Facebook iframe permission; correct stale file-header claim. |
| `.github/workflows/gates.yml` | KEEP `LC_SWEEP_WORKERS=1` fixture isolation: shared-shop global before/after fingerprint must not race another page's copy. Controls/assertions/thresholds/coverage unchanged; ten CI shards remain parallel, each isolated. This does not prove within-shop concurrent click correctness. |
| click-sweep fixtures | KEEP bounded synthetic domain/session leases and authenticated MOCK CommentStream. Restore trunk `PaymentProfile:PROVIDER_MOCK`. No production auth policy or expiry-negative bypass. |
| `catalog-v2-client.ts` (this round) | Optional numeric status observer on the **existing send**, preserving Outcome shape and all callers. Needed to distinguish first inventory 4xx from lost ACK without a second writer/fetch implementation. |
| tests/runner wiring | Keep formal modes, heartbeat locking, serial local PG, actual-click assertions. Only ruling-changed reload retry visibility is updated; add same key/body/one-effect assertions. E2E prompt loop covers all three supported locales, not Japanese. |

## Commands / evidence

| Command | Exit / count | Log under this directory |
| --- | --- | --- |
| `node --test --experimental-strip-types tests/admin/live-workspace-hooks.test.ts` on pre-fix source | **1**, 6 behavioral failures | `review-hooks-red.log` |
| Added in-flight/reload→403 negative before race repair | **1**, durable fence missing | `review-journal-race-red.log` |
| Hook/client tests on repair (`--experimental-transform-types`) | **0**, 20 PASS | `review-hooks-client-green.log` |
| Component SSR normal / A1-poll or notice fault injection | **0** / **1** / **1** | `review-console-render.log`, `review-console-render-poll-red.log`, `review-console-render-notice-red.log` |
| Recommend grammar fault (in-memory boolean admission) | **1**, true must be rejected | `review-recommend-red.log`; normal assertion included in Node green |
| `LC_CONSOLE_A1_FAULT=offers_null bash scripts/dev/test-focused.sh '^TestLiveConsoleLCN27TypeScriptActualA1$'` | **1**, production TS parser rejects null offers | `review-real-a1-red.log` |
| Same focused command, fault **unset** | **0**, real A1/PG **1 PASS** | `review-real-a1-pg.log` |
| `bash scripts/dev/test-node.sh` | **0**, **649 PASS / 0 FAIL** (sum of runners) | `review-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `review-tsc.log` |
| `LC_HEADER_BASE=0813424a bash scripts/dev/check-gates.sh` | **0**, 78 modes; header ratchet and both Go tag-set checks | `review-check-gates.log` |
| `gofmt -l` touched Go tests; `git diff --check` | **0**, no output | author receipts |
| Impeccable detector, changed Console/SessionCopy targets | **0**, `[]` | `review-ui-detector.json` |

Evidence **E3 for local Node/type/static and focused REAL_PG + MOCK route/JSON checks**, not new-source full browser acceptance. Hook dispatcher/SSR inject read states; they are not React DOM/hydration/network or click proof. Read-only source reviewers found no remaining scoped P0/P1; they did not independently run browser gates. Red evidence retained; no failure assertion/threshold deleted.

## CI gates (integrator owns push and completion notification)

Run on the delivered SHA, without focus/calibration envs:

- `bash scripts/dev/test-local.sh --browser-admin-shell`
- `bash scripts/dev/test-local.sh --browser-live-console`
- `bash scripts/dev/test-local.sh --browser-live-claims`
- `bash scripts/dev/test-local.sh --browser-studio-bff`
- `bash scripts/dev/test-local.sh --browser-studio-ui`
- `bash scripts/dev/test-local.sh --browser-click-sweep` (ten shards)
- `bash scripts/dev/test-local.sh --browser-visual-lint`
- Also `--browser-e2e` for the ruling-changed host prompt clipboard matrix, and `--browser-catalog-core` / `--browser-catalog-media` for the shared catalog transport observer.

**NOT_RUN:** above heavy modes/full foundation/Linux browser/new screenshots/provider SANDBOX/LIVE on this source. Optional `tests/media/r04-input-runner.test.mjs` was NOT_RUN by test-node (`COMMERCE_R04_LIVEKIT_BINARY` unset); Node pass count excludes it. No production Go/SQL/migration change, push, deploy, Meta mutation or live keys. No background gate remains running; focused sessions terminated normally and owned PG cleaned up. W2-U2 remains queued until this revised candidate is accepted.

## Source hashes (unchanged by final evidence commit)

| File | SHA256 |
| --- | --- |
| use-live-workspace.ts | `67d1c0429bca441fce7ded0c3546d40d524e8d113404dd37c444a8d0d1a708a0` |
| command-journal.ts | `01edda41f0af6bd88023dd7485b587ca52ff55092e3f1de96c5e8f8cf06ebbef` |
| console-client.ts | `e1b5de4b9768be66acac1a1098ac7603670513ad40b9a7740486685a492717ea` |
| live_console_ts_parity_test.go | `744d0cc18666619a6fde2a0dd6cc1e96e2036b7cd7dc139b952e5aad555082ed` |
| live-workspace-hooks.test.ts | `e992001bc5e759ba1ec81a0a0aaf935bf8f2e7442238ba088d6e248a5981ec27` |
| live-console-render.test.ts | `50a6a14238ff48a65a7fadf254d112160892d5aac714f0dcf5390831986735f7` |
