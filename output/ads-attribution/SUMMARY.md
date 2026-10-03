# ads-attribution — initial checkpoint, NOT RELEASE ACCEPTANCE

## Scope and state

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution`
- Branch: `unit/ads-attribution`; base: `d282c98816a6bcab46d1eb862974608d297dd298`.
- Read D1–D9 and AT1–AT9, AGENTS, PROCESS, DESIGN and relevant symbols. Offline installation exited 0, with no package downloads.
- Latest integrated brief reserves **0113**. The chat says **0112**, already allocated to the preceding ads-tw-regulation unit. An asynchronous clarification was sent; **no new migration has been created**.
- This base does not yet contain prior-unit source `b7afdf1d` (`git merge-base --is-ancestor b7afdf1d HEAD` exited 1). No merge/cherry-pick was performed. Do not accidentally replace the prior unit's refusal-text fields when integrating this unit later.
- **Open P2 — do not wire this cookie prototype into runtime:** independent review found the browser ID lifetime coupled to the seven-day attribution lifetime. After seven days, a new click creates a new `fbp`. D2 requires a stable browser identifier, but does not set its retention lifetime. The exact red counterexample is in `red-fbp-lifetime.log` (exit 1). Separate authenticated browser identity from touch eligibility once the retention rule is confirmed; do not silently invent an indefinite or longer retention policy.
- No push, merge, deployment, credentials access, sandbox execution, production write or Meta mutation.

## Checkpoint commits

- `a0c7d3fc`: preserve ad query through the locale redirect; regression red then green.
- `79eb1643`: isolated cookie prototype plus six unit tests; deliberately not wired to runtime, known retention P2 remains.
- Both commits carry `Co-Authored-By: Codex <noreply@openai.com>`.

## Implemented checkpoint (not wired to runtime)

1. **Locale hop**: `/products/{id}` still redirects to a fixed relative `/zh-TW/products/{id}`, now preserving the query. A red test first proved the old code discarded `lc_ad` / `fbclid`.
2. **Cookie primitive**: `ad-touch.ts` signs a bounded first-party envelope with a host-bound HMAC; validates duplicates, malformed identifiers, signature and plain-cookie agreement, future timestamps and a seven-day age limit. It has no network calls and no PII fields. It is **not imported by proxy or BFF yet**; no visitors receive these cookies from this checkpoint.
3. **Independent test design**: readonly `at_capture_map` supplied 12 negative cases. Its initial incorrect statement that 0110 was unavailable was corrected: `claims.order_live_sources` exists at lines 17–29 and yields order→session only, not exact comment post.

## Commands actually run

| Command | Exit | Evidence / count |
|---|---:|---|
| `pnpm install --offline --frozen-lockfile` | 0 | 49 reused, 0 downloaded |
| `node --test --experimental-strip-types apps/storefront/tests/ad-link-route.test.mjs` (pre-fix) | 1 | 3 tests: 2 PASS, 1 FAIL; `red-ad-link.md` |
| Same command after fix | 0 | 3 PASS, 0 FAIL |
| `node --test --experimental-strip-types apps/storefront/tests/ad-touch.test.mjs apps/storefront/tests/ad-link-route.test.mjs` | 0 | 9 PASS, 0 FAIL |
| `bash scripts/dev/test-node.sh` | 0 | `node-preflight.log`; 336 PASS, 0 FAIL, 0 SKIP; optional R04 binary suite NOT_RUN |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `storefront-tsc-preflight.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-preflight.log`; existing 60 modes |
| `git diff --check` | 0 | Clean |
| Independent-review lifetime counterexample, root reproduced with `assert.equal(next.touch.fbp,first.touch.fbp)` after `AD_TOUCH_TTL*1000+1` | 1 | `red-fbp-lifetime.log`; known P2, intentionally unresolved pending retention decision |

These preflight greens do not satisfy AT1–AT9 or full release G07. Re-run final affected suites after runtime integration.

## Gate ledger

| Gate | State |
|---|---|
| AT1 | PARTIAL: locale-hop regression and cookie pure tests only; proxy/BFF, Go Begin, PG and browser NOT_RUN |
| AT2 | NOT_RUN: CAPI frozen fields / consent / unchanged event_id |
| AT3 | NOT_RUN: comment attribution and boost-window boundaries |
| AT4 | NOT_RUN: per-draft exact order/refund/COD totals and separate Meta figures |
| AT5 | NOT_RUN: full real-click journey, three locales and 390/1586 screenshots |
| AT6 | NOT_RUN: owner dataset/test event code and Events Manager sandbox validation are not available to this checkpoint |
| AT7 | NOT_RUN: full G07 and new-report click sweep |
| AT8 | NOT_RUN: dimensional snapshot replacement and isolation |
| AT9 | NOT_RUN: session timeline, buyer distribution, live audience MOCK; read_insights-dependent LIVE verification |

Go build/vet/gofmt, admin tsc and focused PG were not run (no Go/SQL/admin change in this checkpoint). Full G07 remains mandatory for the final storefront/runtime unit.

## Design boundaries for continuation (not settled rulings)

- **Erasure**: `customers.apply_erasure` (0078:254–295) retains order facts and referenced snapshots for legal retention, unlike D8's assumption. Implement selective removal of advertising pseudonyms / CAPI context, never removal of financial facts. Decide retention/storage of checkout IP separately from the pseudonymous attribution snapshot.
- **Multiple factual paths**: D3 and D4 do not specify a winner if both a recent click and a promoted-post claim exist, or if several live drafts promote the same post. Do not double-count or invent paid-comment precision. Record the chosen tie/ambiguity policy before its SQL and tests.
- **Cookie policy**: the missing-`fbclid` case and browser-ID retention are not fixed by the seven-day attribution rule. Proposed separation: signed browser identity independent of the last-touch age; owner/integrator sets its retention. Until then, the prototype stays unreachable from runtime.
- **Post identity**: the existing `claims.order_live_sources` helper is order→session, not order→comment. Join from specific claims/intake evidence, not all sources of a session.
- **Time axis**: Meta hourly buckets use the advertiser account's timezone; the merchant's report uses Taipei days. Align those explicitly; do not relabel account-local hours as Taipei hours.
- **Live audience**: empty demographics are unavailable/privacy-threshold, not zeros. Age/gender buckets represent view time, not unique people. No buyer demographic join.

## Agent record

- Root Codex: implementation and local verification; sole writer in this worktree. No model override.
- `at_capture_map`: explorer, gpt-6-luna / medium, readonly symbols and negative-test design, same base; no tests, PG or browser. Humaux unavailable to that child; root stored the corrected result.
- `at_insights_map`: explorer, gpt-6-luna / medium, readonly ads/report/live/ACL map, same base; research stored as `ads-attribution current branch symbol map and D9 gaps` (queued).
- `at_cookie_review`: security_reviewer, gpt-6.1-sol / high, readonly cookie/redirect diff review. No confirmed P0/P1; P2 fbp lifetime coupling. Reviewer reran the focused tests: 9 PASS, exit 0. Humaux review ID `0f5bdbe1-6f46-42ac-90e1-81f2c9546901`. Missing/stripped `fbclid` behavior is UNKNOWN in the brief, not a proven bug. No PG/browser/full release verdict.

Skills used for preparation: frontend-architect (separate cookie boundary from transaction authority), impeccable (audit existing reporting structure), playwright (planned real-click gates). No new UI/visual surface has been built or approved here.
