OA-2 freezes the C3/C4 command surface while integrator-assigned SQL and composition paths remain blocked. `LC_ORDER_ATTEMPT_ENABLED` stays `0`.

## RVFIX305 policy handoff fix — implementation CI pending

The reviewed C3 request called legacy `inbox.check_send`, which accepts only `lcn-policy/v1`; OA-1 stores `order-attempt-notification/v1`, so eligible OA READY notifications would be refused. The handoff now requires a narrow OA validator, its exact owner/EXECUTE inventory, and C3 use under the existing lock suffix. Legacy validation remains unchanged; stored policy rewriting is forbidden. UNKNOWN reconciliation still branches before fresh-send eligibility.

The SQL acceptance handoff now requires separate eligible-READY positive, consent refusal, window closure and changed-takeover REAL_PG witnesses. This is a corrected migration request, not an implemented SQL authority or dispatch activation. No migration number has been assigned.

- Assertion RED: head `4a84a5dfe82231fff4b511672a340ab25ddad655`; Gates `40879903464` / g10 `123470232597`, failure exit 1. New authority/ACL/C3 assertions and all four policy witnesses failed at their assertions. Existing consumed-permit witnesses passed. Native synthetic merge `e720aee049612c36894c7a1636ee6cdda1c32e1e`. Filtered native evidence: `output/oa-2/rvfix305/policy-red-job.log` and `red.log`.
- Foundation `40879903437` / `123468328227` passed on the RED head; it does not run this document test. Current implementation GREEN pending; new run/job ids will be recorded in a PR comment without an evidence-only commit.
- Fetched/merged `origin/r3/integration@d0e7331525fec046fbe9361906fa66367ae583c6` first (already current). Browser ownership regenerated last before push.
- No local tests/servers. Diff whitespace check only; document gate awaits native CI. Own assessment E0. Independent delta review pending.

P2 for issue: reminder identity CHECK still permits NULL keys. The earlier delivery's claim that explicit NOT NULL predicates were added was false and is now corrected. Keep P2 open for the integrator's schema task: both explicit predicates plus separate omitted-key REAL_PG negatives before schema adoption.

Scope: this implementation batch touches only `output/oa-2/**`; outside-card files: none. Browser ownership regeneration is card-authorized (commit only if it changes). No migration, contract, OpenAPI, dependency, R2 pin, composition or activation changes. Assertions are unchanged.

NOT_RUN: REAL_PG C3/C4 paired state; actual-login ACLs; lock barriers; M2a/M2b/M4/M7 DB mutations; scheduler/cancel/rotation/copy/reminder composition; provider I/O; OA-3 cutover; SANDBOX/LIVE; production. Humaux/coord tools are unavailable, so no store/canvas update was performed. The integrator relay resumes this worker when native CI completes; no local background job is running.

Rollback: revert this documentation implementation commit on `unit/oa-2`; no DB effect exists.

🤖 Generated with Codex (dispatched by Claude Code integrator).
