# PR7 browser timezone independent review

task_id: 5a3751be-c0ad-4668-b941-036a6feaf8ce-sub-review
base_commit: 5176e490f79a820fb28d36f6ac5095141eaa28c5
role: readonly independent test reviewer; model: gpt-6.1-sol; reasoning: high
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/tz-audit
change_paths: only output/tz-audit/browser-timezone/review/**; no source writes, claim, recursive delegation or push.

Initial P1: tests/admin/ads.spec.ts:354-355 reload expects expanded selected draft, but apps/admin/components/Ads.tsx:300-304 create callback only writes selected state; only the row button (455-459) writes draft query. The initial openAds URL has no draft. A test must pin the query by real row close/reopen before reload, or click the collapsed row after reload.
Initial P2: tests/storefront/claim-checkout.mjs:74-79 reused page.seen.preview can accept the prior preview while reopening the same link. Reset and capture a new GET before navigation, then use that DTO for expected/persisted expiry.
No P0 source issue found. Runtime acceptance remains NOT_RUN.

Verified source: design loadAll reads versions on mount; publish refreshes versions after successful POST; td nth(2) is published timestamp. Ads CreateDraft returns full ads.draft_view DTO. Manual receipt expires_at and transfer deadline_at source the same checkout order expiry; receipt is ephemeral state, durable buyer deadline is reload surface. Claim reload reopens the same issued link rather than storing token. UTC contexts reuse existing session/orders; no additional fixture/order/schema. Order history has two exact rows and exact time text; transfer proof and deadline use positive date+label assertions. Existing Go PG assertions, screenshots, thresholds and failures unchanged.

Evidence: static-commands.json (six node syntax commands and git diff --check, each exit 0); spec-list-command.json and spec-list.log (synthetic-environment Playwright --list, exit 0, no app/browser/network; 13 tests including one synthetic design journey); SHA256SUMS binds the source snapshot.
Evidence level E1. Browser mutation, full six modes, Go/PG/provider sandbox/live: NOT_RUN by reviewer. Root owns repairs and runtime acceptance. No owned process remains running.

Final source recheck: root repaired ads.spec.ts:354-360 with real close/reopen, explicit draft URL assertion, then reload; root repaired claim-checkout.mjs:74-81 with preview reset and dedicated fresh GET matching the actual X-Commerce-Claim-Token header. Initial P1/P2 are RESOLVED_SOURCE; no remaining P0/P1 found in bounded source review. Six syntax commands, diff check and synthetic Playwright list rerun all exit 0 (static-commands.final.json/spec-list.final.log). Final source hashes in SHA256SUMS; initial evidence retained. Runtime acceptance still NOT_RUN.

UTC reread follow-up: ads.spec.ts:340-349 retains same store/draft query, safely permits harness TLS, and repeats same server-derived date text after reload. design.spec.ts:291-303 retains same j.id/session/publishedAt and clicks Versions after reload; production design harness origin is HTTP, so its context needs no TLS exception. manual-order-link-gate.mjs:222-231 now uses UTC for existing regenerated link, confirms original body.order_id and original expiry before/after reload; no new link creation beyond pre-existing regenerate action and no new order/fixture. Browser clocks remain real; dates are captured from actual Go UI-produced DTOs, not frozen timestamps. All four targeted static commands exit0, final manifest refreshed; prior manifests retained. No remaining P0/P1 found. UTC admin contexts do not explicitly start tracing: do not claim a UTC-specific retained trace without observing the runtime artifact. Runtime/mutation results remain NOT_RUN by reviewer.

RED proof review 2026-10-08: actual archived trace confirms America/Los_Angeles browser, addInitScript deletes only Intl options.timeZone, real Publish click and successful server versions GET containing v1 published_at=2026-10-08T00:03:48.29855+08:00.
Playwright log contains 4 real timestamp assertion failures (Taipei 10/08 00:03 vs LA 10/07 09:03) and 4 unrelated passes; Go runner exit1. Later history-count failure follows early journey abort, not an independent runtime defect finding.
Restored proof spec equals main exactly; Node shared format helper equals main; all six source hashes equal the prior final manifest. No source/runtime changes by reviewer.
GREEN attempt exited -15 with no browser log; proof-green-not-run.json truthfully retains NOT_RUN. This proves RED sensitivity only; normal six browser modes and restored GREEN still require GitHub evidence.
Artifact/source hashes and readonly command evidence: proof-readonly-review.json. Reviewer ran no browser/Go/PG tests; read commands exit0.
Stale DELIVERY.md RUNNING wording reported to root for correction. No new P0/P1 source finding; full acceptance remains pending.
Delivery recheck: root refreshed DELIVERY.md:72 to actual four-date RED/four passes and normal GREEN NOT_RUN; stale wording observation resolved. No further source change or test run.

Final nine-line hardening review: ads hint assertions target the actual AdsDraft fDatesHint; manual deadlines repeat UTC+8 labels after added reloads; UTC order history arms a fresh response before the real post-reload click and reasserts the same two orders. No new fixture/write or store-scope change. Manual Purpose/Depends on/Used by header included in final hash.
Changed two MJS node --check and git diff --check exit0; focused strict TS ads+design exit0 with explicit existing apps/admin Node typeRoots. Initial generic tsc exit1 due absent auto-discovered Node types retained in static-commands.label-hardening.json; corrected command recorded separately, no dependencies/source changed.
SHA256SUMS refreshed all6. Design81e6a55f unchanged, so actual archived RED proof remains valid for design only; runtime evidence does not extend to these new label/reload assertions. Normal six-mode GREEN remains CI NOT_RUN; source review has no new P0/P1.
