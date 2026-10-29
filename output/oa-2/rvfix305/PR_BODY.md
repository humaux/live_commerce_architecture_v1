OA-2 freezes the C3/C4 command surface while the integrator-assigned SQL and composition paths remain blocked. `LC_ORDER_ATTEMPT_ENABLED` stays `0`.

## RVFIX305 — assertion RED in progress; do not merge

Review found that the original RED stopped at compilation and that the C3 SQL handoff applied fresh-send refusal before discovering UNKNOWN reconciliation mode.

- This push batches compilable, explicitly marked mutations against all nine unchanged model tests. A new document-order gate rejects the defective C3 handoff and requires separate cancel/rotation/policy-loss-after-UNKNOWN acceptance witnesses.
- After CI reaches the intended assertions, restore the mutation source from `aace3b5a50a660be6f6f1f337214aacadce84aea`, fix the SQL handoff at its state branch, and make one implementation push. No current-head GREEN is claimed.
- Merge of `origin/r3/integration@f8fd89005` precedes this batch; browser ownership is regenerated last before push.
- Static E1: gofmt and `go vet ./internal/integrations/metareply ./tests/foundation` exit 0. No local tests/servers. Own assessment E0.

Historical receipts: API-absence RED `40844363540` / job `123348127167` did not reach assertions. MODEL_ONLY GREEN `40849140762` / job `123363948641` and Gates `40849140547` / required `123379645345` belong to `5edaaa082`, not the current head. Native current-push run/job receipts will be posted in a PR comment.

P2 for issue: reminder identity CHECK permits NULL keys; explicit NOT NULL predicates and omitted-key real-DB negatives are required before schema adoption. See review finding at output/oa-2/SQL-REQUEST.md:24.

Scope: original OA-2 write paths plus generated output/ci-select-backend-browser/browser-ownership.json, explicitly required by the card. No other out-of-scope edits. No migration/contract/OpenAPI/dependency/R2-pin/composition changes. Inherited integration files come only from the requested merge.

NOT_RUN: REAL_PG C3/C4 paired state, actual-login ACLs, lock barriers, M2a/M2b/M4/M7 DB mutations, scheduler/cancel/rotation/copy/reminder composition, provider I/O, OA-3 cutover, SANDBOX/LIVE and production. Document checks do not prove SQL runtime behavior. Independent delta review remains pending.

Rollback: restore the marked mutation source after assertion RED; never merge a mutation head. Evidence: output/oa-2/RVFIX305-DELIVERY.md and output/oa-2/rvfix305/red.log.

🤖 Generated with Codex (dispatched by Claude Code integrator).
