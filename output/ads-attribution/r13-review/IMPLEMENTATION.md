# R10 + R11 implementation and evidence boundaries

Final frozen source: `4acbad5353814fc3940812e9e399fdb1253ed40f`.
Pre-browser-fix checkpoint `40128fe5db6f04c8284ddc74fe2a302ba628900d` passed
PG/static checks, but its READY-only UI parser rejected an actual R11 SUCCEEDED
replay. Final source includes strict acknowledgement states and honest persisted
UNKNOWN feedback; `0c5f93f3` adds fresh per-case sources while retaining completed-id
replay checks. Final gate status is recorded in `../SUMMARY.md`, not inferred from
that earlier checkpoint.
Authorized integration merge: `8f419664` includes `373462d4` (R10 + R11).
Migration remains **0113**; 0112 refusal fields are unchanged. No push, deploy,
Meta mutation, sandbox send, production credential read or buyer PII was used.

The handoff evidence commit also includes this worktree's generated
`output/platform-site/server-{tag,no-tag}.log` and `output/ui-click-sweep/`
journey/ledger files, written by the final click-sweep gate. They are test
artifacts, not platform-site or application source changes. Final runtime source
remains `4acbad53`; no source was edited during or after its full G07.

| Ruling | Implementation | New/retained acceptance |
|---|---|---|
| R10 P1 | Paid-only order, path, buyer and timeline counts. Pending COD/pickup stays separate; unpaid/draft/expired/cancelled does not become a collected order. | Independent nine-order mixed cohort, actual Begin and exact PG/browser numbers. |
| R10 P2-1 + R11 privacy | Begin may freeze valid host-bound fbc/fbp/IP without a seven-day touch, but only with ads_personalization consent. Signals-only rows have NULL path and are excluded from report facts. | Cookie/BFF RED→GREEN, consented/no-consent actual Begin, paid signals-only report negative. |
| R10 P2-2/P3 | Insufficient audience, non-Taipei Meta account day, provisional and promoted-post labels; ROAS two decimals via packages/format; stable same-fbclid timestamp. | Six locale/viewport real-click report cases; pure formatter/cookie tests; actual pinned Caddy XFF/XFH/Host regression. |
| R11 I23 | Fresh auth, exact source and current read_insights checks precede a per-store/video transaction lock. Existing in-flight or final-within-10min operation replays with no extra River job. Fresh job stays in the outer command transaction. | Eight concurrent distinct HTTP keys, exact operation/job counts, final replay/cooldown, real source transfer, unchanged lease/authority negatives. |
| R11 I12 | Absent daily snapshots yield NULL spend/ROAS. Omitted optional metrics and local-only timeline spend remain NULL, never fabricated zero. | PG empty draft/session; adapter omission cases; strict model and six-matrix reload assertions. |
| R11 optional failures | Read each dimension independently; malformed/4xx/5xx/DST-invalid dimension is unavailable while D7 daily and other dimensions succeed. | Adapter RED→GREEN and PG malformed age×gender with valid hourly data. |
| R11 bounded retries | Existing Insights lane/hour key; at most four additional unavailable account-days, oldest fetched status first, non-extending 24h deadline. A final daily row cannot be overwritten. Status-row guarded UPSERT serializes late older completions. | Actual next sweep outside D7, four-day cap, immutable daily equality and first-failure deadline. |
| R11 privacy terminal states | All three send-once identifiers are cleared at final CAPI states including UNKNOWN. UNKNOWN never resends. Withdrawal follows the existing 0080 bounded-purge pattern; erasure clears synchronously. | Terminal/UNKNOWN/no-operation/consent/erasure negatives and 1000 retained rows before a withdrawn target (no starvation). |
| R11 SQL email boundary | SQL normalizes/hashes the email; worker receives only email_hash under exact CAPI lease/policy gates. | SQL signature and normalized hash assertion, unchanged payload hash. |
| R11 copied lc_ad | Click time must lie in a successfully activated draft window; paused/ended/never-launched links are not credited. | Real activation plus before/during/after/never-launched cases; browser fixture truly activates its MOCK ad. |
| R11 report bound | First 100 drafts/sessions, 101st sentinel sets truncated; UI says only first 100 shown. | 100/101 drafts, 101 sessions, six-matrix cap notice; 10k paid-order cold HTTP below existing 5s deadline. |
| R11 session spend | Label explicitly refers to ads promoting the session's posts during the selected report period. | Three-language exact text and unknown spend/derived ROAS reload checks. |
| R11 Page scope | Current token optional read_insights is preserved and rechecked. Missing grant yields reconnect state/link; base Page eligibility remains its frozen four permissions. | Mock scope pass-through/decline tests; real-click settings link. **Remote config_id update is not performed.** |
| R11 integrity | plan_capi_purge replacement needle is asserted; contract lists every new EXECUTE; domain table access remains behind narrow definers. | Exact schema/ACL/upgrade gates, independent SQL review. |

## Rejected experiments and fixture corrections

- Nested savepoints cannot be used around a fresh River job: its subtransaction
  xmin fails the existing same-transaction planner guard. Use authenticated NULL
  preflight under the same outer transaction lock, then create only when needed.
- No speculative query/index/JIT tuning remains. A target-cohort CTE experiment
  returned 3.108s vs the original query's 2.370s on the same ANALYZE fixture, so it
  was removed. Earlier >5s runs lacked statistics after the synthetic bulk load.
  ANALYZE does not run or warm the report; the timed HTTP remains the first read.
- New test fixtures initially violated existing email whitespace, unique active
  source, claim-window FK and one-open-window constraints. They now use a valid
  mixed-case synthetic email and real deactivate/register with a CLOSED window.
  No schema, guard, assertion or deadline was relaxed.
- Optional retry originally selected by calendar day; the real aged-retry test
  showed eligible statuses could be skipped behind repeated old failures.
  Selecting oldest fetched status first provides bounded rotation.
- Final mixed-cohort fixture verification uncovered three independent setup
  errors: stock was replenished before claim creation instead of actual Begin;
  the collection/release HTTP routes were never mounted; and the second ad was
  paused before its first Insights read, outside D9's 04:00 paused-ad polling hour.
  The fixture now uses the real inventory API immediately before Begin, the real
  collection handler with external logistics disabled, and captures both real
  MOCK Insights snapshots before its genuine pause. Amount/count assertions are
  unchanged. The R11 NULL-spend timeline contract now has an explicit unknown
  assertion alongside the original exact 1730 known-spend aggregate.
- T06's exact definer inventory explicitly adds the R11 private boolean helper
  (69 → 70), with all original ownership/EXECUTE/PUBLIC assertions retained.
- The report browser exposed a real acknowledgement mismatch: R11 can replay a
  completed SUCCEEDED operation, while the previous UI parser only accepted
  READY and displayed a transport-unknown retry. A strict known-state parser
  and honest persisted feedback are required; broadening the test to arbitrary
  success strings is rejected. Each locale/viewport gets an unrequested source
  to retain the exact first READY assertion, plus the pre-completed main source
  for same-id SUCCEEDED replay. PG counts reject duplicate operations.
- Root `0c5f93f3` supplies six real, previously unrequested audience sources;
  root `0f1ee6eb` imports author `72f36edb`'s strict nine-state receipt parser,
  tri-locale honest feedback and persisted journal. Transport UNKNOWN retains
  the original HTTP key; an explicitly acknowledged subsequent read may use a
  new HTTP key, while server I23 reuses the same operation. The real dispatcher
  is not frozen: each fresh source asserts its first READY response only; the
  pre-completed main source asserts two SUCCEEDED responses with the exact same
  operation id. No arbitrary success-state union replaces the frozen assertions.
- Independent receipt review `34d69c85-36c2-49b5-b2d7-c9cd9230aed9` found a
  misleading same-POST progress action for an authoritative cached UNKNOWN.
  Root `4acbad53` (author `62a77c98`) removes that action, keeps new requests
  disabled, and preserves transport-UNKNOWN same-key retry. Three-language Node
  RED 4 failures → GREEN 8 passes. Independent follow-up `38111522-a424-44f1-bb52-e5d609143e35`
  is CLEAR; it is source review, not a substitute for root gates.
- Independent visual review of the real 100-draft fixture found a mobile UUID
  text wall. Root `52674e06` uses native closed details with the full count,
  complete selectable IDs, 44px touch target and keyboard focus. SSR RED 3 →
  GREEN 17; browser compares expanded IDs exactly with the real server response,
  then closes using Enter. No report facts or cap assertions were removed.

## Independent work and ownership

All children used separate worktrees, no recursive delegation. Root imported
reviewed commits; children did not write root product code.

| Role / task | Base / delivery | Allowed paths | Model / effort provenance |
|---|---|---|---|
| integration_worker r11_adapter | 8f419664 → 7fd16afe (root d2567048) | meta_ads breakdown/ops tests and metaconnect graph/scope tests | Actual runtime identifier/effort unavailable; author records UNKNOWN |
| ui_worker r11_ui | 8f419664 → 42c3d5f9, c47b0c85, 7687fb86 | Attribution component/model/copy and corresponding Node/browser tests | Actual runtime identifier/effort unavailable; author records UNKNOWN |
| test_worker r11_pg | 8f419664 → 0dd21862, c2e5f546, 0d83b8f6 | four explicitly assigned foundation test files | Requested gpt-6.1-sol / high per parent launch; runtime not independently exposed |
| security_reviewer r11_sql_review | read-only 85cfb147 and narrow follow-ups | no edits | Requested gpt-6.1-sol / high per parent launch; runtime not independently exposed |

Author evidence: `author-pg/`, `author-ui/`, and `../r11-adapter/`. Root does not
substitute compile-only or author evidence for independent final PG/browser/G07.
Frontend-architect kept strict transport models and the shared formatter; the
Playwright workflow uses actual production Next builds, real PG and clicks,
with only provider edges mocked.

## Explicit NOT_RUN / external prerequisites

- AT6 SANDBOX: owner's dataset/test-event code unavailable/not authorized here.
- AT9 LIVE: actual live-video read_insights approval/grant and suitable live
  evidence unavailable; current unit makes no external Meta mutation.
- Owner/integrator must add optional read_insights to the existing Meta Login for
  Business configuration selected by config_id, then reconnect and attest it.
  App-role limitation before App Review remains. A URL scope override would
  violate the frozen Page OAuth contract and is deliberately not used.
- Optional R04 runner Node suite needs COMMERCE_R04_LIVEKIT_BINARY; missing
  prerequisite is reported by the standard script, not treated as a pass.
- Accepted P3-10 owner-definer RLS pattern and P3-12 order-level approximations
  remain documented; no buyer age/gender is collected and no Pixel is installed.
