# R12 round 2 — source and regression evidence

Initial frozen source was `2817f4ce0f33dd1bf85d1a733b6c4f51adb7f04b`.
Its browser gate found unstable server draft-ID ordering; it is now a historical
checkpoint, not the final acceptance source. Final source is
`6c6c3fb397dd8c6ae8c291f3d751c1eedbc6ac89`; final logs live in `../r12-final/`.
No runtime or test source changes are allowed during those final gates.
The parent SUMMARY contains the final gate results.

## Logical commits

- `506e5f530a93b92b3b6583326b5e8cb6a2b82157`: authorized merge of R12 ruling
  `664db345` from integration; preserves 0112 refusal fields.
- `a33bf127d3519ebeb10be55fb7f1a1943003f3fc`: R12 contract regression tests,
  real order/consent/capture fixtures and six-case browser coverage.
- `2817f4ce0f33dd1bf85d1a733b6c4f51adb7f04b`: fixes, contract clarification,
  replay privacy assertions and SQL alias repair.
- `6c6c3fb397dd8c6ae8c291f3d751c1eedbc6ac89`: deterministic linked draft UUID
  aggregate order; strict browser assertions unchanged. Independent read-only
  review found membership, scope and downstream calculations unchanged.

## Required changes

1. **F1 cohort:** remove the signals-only exclusion from the order cohort.
   NULL-path rows still do not enter path totals or draft credit. The regression
   uses an actual signed comment on an unboosted post, claim redemption,
   consented card Begin and signed Stripe capture. The session reports exactly
   one paid organic order and its net income, with no credited draft/path.
2. **F2 GET recovery:** READY replay requires the matching nonterminal River job;
   DISPATCHING replay requires an unexpired operation lease. UNKNOWN and finals
   use only the 10-minute cooldown from `updated_at`. A real UNKNOWN read is
   replayed within the window, then a fresh GET plans after the window.
3. **F3 grant state:** only missing stored `read_insights` shows reconnect.
   Granted but unread shows localized `not_read` and a read action, including
   after refreshing an acknowledged UNKNOWN. A transport-unknown response still
   retains its receipt fence and same-key recovery.
4. **P3 minimization:** only consented card orders can freeze fbc/fbp/client_ip.
   COD, bank transfer and pay-at-pickup retain factual attribution but never
   these CAPI-only identifiers; same-key replay is also checked. Checkout-side
   SECURITY DEFINER ownership, search_path and exact EXECUTE ACL are pinned.

READY has no operation lease until dispatch in the existing River contract.
The fix therefore checks the actual queued job rather than inventing a queue
TTL that could permit both an old delayed job and a new job to dispatch. A
cancelled/completed job cannot pin READY forever. No transaction marker, new
migration number, client PII field, provider write or permission expansion.

## Red → green provenance

| Regression | Genuine RED (exit 1) | GREEN |
|---|---|---|
| Organic signals-only session cohort | `red-pg-cohort.log`: paid orders 0, expected 1 | `green-pg-r12-alias-fix.log`; final `focused.log` |
| UNKNOWN >10 min / granted unread | `red-pg-audience-corrected-fixture.log` | Same targeted and final PG |
| Terminal READY job / expired dispatch lease | `red-pg-cohort-lease.log` audience subtests | Same targeted and final PG |
| Three offline methods never retain signals | `red-pg-all.log` payment subtests; card positive control passed | Same targeted and final PG |
| Exact checkout EXECUTE ACL | `red-pg-acl-mutant.log`: temporary extra commerce_runtime EXECUTE rejected | Same targeted and final PG |
| Strict not_read UI model and actual read control | `red-node-grant.log` | `green-node-final-preflight.log`; final `test-node.log` |
| Acknowledged UNKNOWN allows fresh read intention | `red-node-unknown.log` | Same Node and final real-click browser |

The ACL mutant existed only in a disposable local PG test. Its single GRANT was
removed before the initial frozen source commit. Migration 0113 SHA-256 at 2817f4ce:
`c5a75eec68bee33631db904d6afb5be04f8133790d1f283de10b745181e6eecc`.

Targeted pre-freeze green: PG 9 top-level PASS, 0 FAIL, 0 SKIP (exit 0,
91.344s); Node 28 PASS, 0 FAIL (exit 0). These do not substitute for final gates.

### Preserved diagnostic failures, not falsely counted as contract RED

- Initial organic fixture attempts had a nil comment timestamp, competing test
  consumer keyrings, or a missing SETOF alias. Their logs remain historical.
- `red-pg-audience.log` expected completed for a bounded UNKNOWN job, while the
  real dispatcher correctly cancels it. The corrected fixture produced the
  genuine old-code cooldown failure recorded above.
- `green-pg-r12.log` found SQLSTATE 55000: local record `j` collided with the
  new queue alias `j`. `diagnose-planner.log` records it. The final fix names the
  SQL alias `queued_job`; actual PG rerun passed. No assertion was weakened.
- `browser-attribution.log` at 2817f4ce: EN 390/1586 passed, TW 390 failed the
  full linked-ID array equality; three later cases did not run. Initial and
  subsequent report reads reordered the first three of 103 IDs because SQL
  `array_agg(x.id)` had no ordering. `6c6c3fb3` adds `ORDER BY x.id` inside
  that aggregate; no test/assertion changed. Exact original evidence:
  `output/playwright/ads-attribution-report/20261004T065725.296754000/`.
  Buyer checkout passed. All final gates are restarted on the new source;
  no G07 was launched for 2817f4ce.

## Independent review and ownership

All writers stayed in `.worktrees/ads-attribution`, branch
`unit/ads-attribution`. Root owns implementation and acceptance execution.
Root is Codex; exact underlying runtime model/effort is not exposed, so it is
not inferred. Allowed writes are this worktree's R12 UI/SQL/test/contract files
listed in the logical commits, plus its delivery evidence; no other checkout.
Two non-author subagents were read-only, used no extra worktree and made no
source edits or provider calls; neither delegated further:

| Agent / role | Configured model / reasoning | Scope |
|---|---|---|
| r12_lease_audit / explorer | gpt-6.1-sol / medium | queue/lease/cooldown and read-state implementation review |
| r12_fixture_audit / security_reviewer | gpt-6.1-sol / high | true organic cohort, payment privacy, ACL and six-case browser test review |

Both started from merged `506e5f53`; follow-up reviews examined `a33bf127` and
the runtime diff. The latter identified replay privacy and adversarial ACL
evidence gaps, both closed before `2817f4ce`. Static review did not catch the
SQL variable collision; REAL_PG did. Reviews were stored in Humaux before return.
These are bounded independent source/test reviews, not a claim that reviewers
personally ran all acceptance gates.

Follow-up on final `6c6c3fb3`: the lease reviewer independently approved the
single UUID ordering change; the fixture reviewer verified six click ledgers
(102 PASS actions), actual key-state screenshots, replay privacy assertions and
the ACL negative control. Both reported no new blocking finding. Final migration
0113 SHA-256: `60105665332a7afea78291ee1efed91c34d27969c23475dcaf64ab5fbc7d01be`.

Humaux incremental indexing processed the 14 changed TS/Go source files. SQL
was submitted but the indexer returned zero SQL entities; no SQL graph coverage
is claimed. The R12 decision is linked to AttributionAudienceRead and the
payment/cohort regression test entities; contract and commit references retain
the SQL rationale.
