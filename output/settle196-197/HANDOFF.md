# SETTLE-196-197 relay 2 — implementation pushed, CI acceptance pending

PR #286 into r3/integration; branch unit/settle196-v2-ui; assigned worktree only.
Base merged: origin/r3/integration 313c2c36f (merge d3b98cf64). No merge to integration, auto-merge or force push authorized/performed.

## Evidence
- Tests-only repair head 18bd6a6629815af7844847e8a96ecc6e31c4e863.
- Causal RED: Gates run 40759956329 / unit job 123075243384. New full-manifest-on-every-chunk assertion fails at chunk 0 (settlement_unit_test.go:117).
- Tested synthetic merge be7e4ed536f81f706e0dc3d24f2c6c24506f32f6. Receipt red.log / relay2-unit-red.log.
- RED-head Foundation 40759956153 / 123068875365 passed. Historical only.
- #196 and failed-tail #197 database counterexamples were still running in old run 40759956329 / catch-all g09 job 123075243146 when preparing implementation. No database RED claimed. The relay explicitly requests implementation after the new unit assertion RED.
- Local gofmt, diff whitespace check and permitted packet structure check exit 0, E1 only. No local tests/servers/PG/containers/browser.

## Implementation
0208 (assigned by integrator) adds immutable items_complete on comparison rows and replaces record/close functions in place using guarded exact-shape patches. Compare full manifest to effective ledger IDs/currency/stored nets, excluding unattributed history when a line exists. Every Registrar chunk carries the full manifest/window. Incomplete prefix becomes latest dirty observation; failed final transaction cannot expose old clean coverage. Close checks completeness before writes. ACLs, locks, return carry, scope and idempotence retained; 0200 unmodified.
Existing 0200 aggregate-only comparisons default incomplete and require resync; pre-0200 grandfather rule unchanged.

## Integration prerequisites and scope
R2 pin explicitly set to expected final 138 as assigned; this base plus 0208 has only 136 until 0206/#262 and 0207/#265 arrive. Do not weaken/re-pin locally just to green. Integrator serializes merge after #265 and re-pins at merge time.
Additional path for integrator approval: tests/foundation/stripe_live_schema_test.go holds back 0208 with dependent 0200 in historical pre-Stripe fixture and raises exact Apply count 9 to 10. No assertion removed/weakened.
Generated browser-ownership.json refreshed with canonical static generator for updated foundation hashes. No imports/dependency graph changes, so depmap content unchanged.
Task receipts/handoff in output/settle196-197. No UI/Figma work.
Humaux/code-index tools unavailable; no shared memory/canvas/coord writes. Self-assessment E0, static structure E1, GREEN/REAL_PG and independent acceptance PENDING.

## Resume
Read PR current head and its own Gates/Foundation run IDs from the final relay/PR comment. Read failed jobs only; at most two targeted repairs per blocker. Do not claim pending/skipped checks as PASS. No local test execution. CI queue handoff is bounded to ~10 minutes; integrator relays the next session upon run completion. No local background processes remain.
