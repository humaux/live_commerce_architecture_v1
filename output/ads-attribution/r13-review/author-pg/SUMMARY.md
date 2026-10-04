# R11 PG test author handoff

task_id: c0fcf079-c265-45b0-a738-4c44eb9255af

base_commit: 8f419664f5a2c82085daadce68ab41e31851696b

role: test_worker. Actual runtime model and reasoning: UNKNOWN (not exposed by this child runtime).

worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r11-pg

branch: unit/ads-attribution-r11-pg

Followup task_id: dbe3f4cb-96d8-4145-8526-6f03448021e2. After the initial two commits, cherry-pick 0d83b8f6 (source lifecycle, activated checkout browser fixture, session JSON contract alignment, realistic bulk planner statistics). Root owns subsequent R11 instrumentation and independent GREEN.

Cherry-pick only 0dd21862d0e6dd6f074002ba7dd7d222d743352a then c2e5f54666bcbbc7dc63fb6201daca531a7447b8. Inherited 3579c0dc and 9223d740 are copies of root's e31c610e and 5e4b7afd, not new child changes.

Changed paths:

- tests/foundation/ads_attribution_r11_test.go (new)
- tests/foundation/ads_attribution_session_test.go
- tests/foundation/ads_attribution_report_test.go
- tests/foundation/browser_ads_attribution_test.go

Implementation: eight independent R11 counterexamples cover concurrent distinct-key audience replay, job growth, same-video replay after actual old-source deactivation/new-session registration and ten-minute final cooldown; real activation click windows; explicit null draft spend/ROAS and session spend (session ROAS is derived in the UI); 100/101 draft/session caps; cold five-second API and runtime SQL measurements on 10,000 disclosed synthetic COLLECTED COD projection rows attributed across 100 private-store drafts; optional Graph breakdown failure isolation and omitted nullable metrics; purge starvation behind 1000 eligible rows; four-day aged retry cap, non-extending 24-hour lifecycle and immutable final D7 row. Product source, migration, permissions and acceptance thresholds were not edited.

Legacy AthenB/BthenA snapshot monotonicity assertions remain. The fixture first completes a genuine read and ages the task-owned clock past cooldown, then recreates a pre-R11 overlapping operation by disclosed scoped owner writes. Ordinary new requests remain bounded.

Real browser fixture now provides unknown_draft_id, truncated:true, breakdowns_unavailable:[{day,dimensions:["age_gender"]}], and unknown_breakdown:{dimension:"hourly",bucket:"13:00:00 - 13:59:59"}. The actual Graph response omits hourly reach/impressions/clicks/engagements/comments; spend remains 1230. An explicitly malformed optional age label must not block D7 or hourly data. Cap history is task-owned synthetic SQL; selectable unknown draft is created through the real API. No report interception. integration:read is granted only to the synthetic browser merchant for the real reconnect-settings click.

All commands below ran from the named worktree. REAL_PG uses the repository's serialized disposable focused PG harness and local MOCK providers; none is SANDBOX/LIVE or production acceptance.

| Actual command | Exit | Evidence file | Result |
| --- | --- | --- | --- |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11' | 1 | red-baseline.log | PASS0 FAIL6 SKIP0; two initial fixture errors retained |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11(AudienceBoundReplay\|ClickLiveWindow\|UnknownDraft\|ReportCapAndDeadline\|SessionCap\|UnknownClearsAllSignals\|SQLHashesEmail)$' | 1 | red-corrected.log | PASS0 FAIL7 SKIP0; includes root-authored privacy/hash RED |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11OptionalBreakdownAndNull$' | 1 | red-optional.log | PASS0 FAIL1 SKIP0; unrelated shared-report inventory fixture error retained |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11(ReportCapAndDeadline\|OptionalBreakdownAndNull)$' | 1 | red-final-fixtures.log | PASS0 FAIL2 SKIP0; genuine product deadline and optional-data failures |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11PurgeNoStarvation$' | 1 | red-purge-starvation.log | PASS0 FAIL1 SKIP0; withdrawn buyer's row starves |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11AgedRetryAndFinalDaily$' | 1 | red-aged-retry.log | PASS0 FAIL1 SKIP0; historical clock fixture initially hit frozen draft guard |
| LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-focused.sh '^TestAdsAttributionR11AgedRetryAndFinalDaily$' | 1 | red-aged-retry-corrected.log | PASS0 FAIL1 SKIP0; pre-R11 SQL lacks ads.insights_breakdown_status (42P01) |
| GOTOOLCHAIN=go1.27.1 go test ./tests/foundation -run '^$' -count=1 | 0 | compile-delivery.log | Compile only: no tests run |
| GOTOOLCHAIN=go1.27.1 go test -tags browser ./tests/foundation -run '^$' -count=1 | 0 | compile-browser-delivery.log | Compile only: no tests run |
| GOTOOLCHAIN=go1.27.1 go test ./tests/foundation -run '^$' -count=1 | 0 | compile-followup.log | Final 0d83b8f6 compile only |
| GOTOOLCHAIN=go1.27.1 go test -tags browser ./tests/foundation -run '^$' -count=1 | 0 | compile-browser-followup.log | Final 0d83b8f6 browser compile only |
| git diff --check | 0 | tool result | No whitespace errors |

Red measurements from the private-store paid fixture before the planner-statistics followup: 100 drafts / 10,000 orders HTTP503 in 5.002119667s; 101 drafts HTTP503 in 5.001015334s; subsequent warm runtime SQL returned in 2.599601917s. Followup 0d83b8f6 now calls owner ANALYZE checkout.orders and orders.order_attribution after the bulk transaction, populating normal statistics without running the report. That final fixture's cold measurement is NOT_RUN by the author; root will rerun. No report warmup was added and the five-second threshold remains fixed. On 101 drafts, the deliberate unknown row displaces the oldest paid draft (100 disclosed orders); successful capped totals must be 9900, while the 100-draft case must exercise all 10000.

Unresolved: root's independent GREEN is NOT_RUN by this author. Complete real browser click acceptance, full G07, SANDBOX and LIVE are NOT_RUN. The measured cold API deadline is a product blocker until independently corrected. Missing status-table RED proves only the old schema boundary; downstream retry branches await root GREEN. Normal and browser compilation cannot substitute for acceptance.

No child agents, deployment, push, Meta calls, production credentials or real PII. Harness-owned disposable PG containers/processes self-cleaned after each run. Every failed log remains in this main-checkout evidence directory; root's KEEP/data and other tasks were not deleted.

Humaux: stored "R11 independent PG author counterexamples 0dd21862" (memory 304b4fbd-7335-4996-abc0-9e36481c31c2) and "R11 PG author final RED delivery c2e5f546". Canvas r11_pg updated; author-only coordination task completed. Code indexed and TestAdsAttributionR11AudienceBoundReplay linked to the rationale memory.

Followup stored as "R11 author fixture followup 0d83b8f6 frozen lifecycle stats and checkout"; followup task and canvas updated. Latest changed files re-indexed. Worktree clean; no author-owned test processes remain.
