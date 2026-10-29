OA-2 freezes the C3/C4 command surface and SQL migration handoff. Executable SQL/composition remain blocked on integrator assignment; `LC_ORDER_ATTEMPT_ENABLED` stays `0`.

## RVFIX305 independent review: both P1 handoff corrections

OA-1 creates the payment-DM operation and READY outbox together. The old C3 request counted that operation as its own recent duplicate, voiding a valid immediate claim. The request now scopes the predicate by tenant/store/order/action and excludes `p_operation`; it pins an immediate eligible claim, a distinct recent-operation refusal, and cross-scope/order isolation witnesses.

The old schema request could not persist its own consent_refused or default no_effect dispositions. It now replaces the restrictive void_code CHECK, retaining all 14 OA-1 codes and adding cancelled, rotated, consent_refused, invalid_request and no_effect. Fixed helper-to-paired-code mappings precede both writes. Consent refusal and authoritative-no-effect witnesses require successful commit under that CHECK plus raw paired readback; UNKNOWN alone never proves no effect.

This implements the review corrections in the SQL request, not deployed SQL. No migration number, executable migration, shared schema, contract, R2 pin, dependency, composition or activation change is included. The P1 tests from the tests-only RED push are unchanged.

## CI evidence

- Tests-only RED head: `76497fd0ee3ccf57bd49585892b651c4f6e1ed2b`.
- Gates `40895129665`, g10 job `123524293876`, attempt 1, failure / exit 1. Tested merge: `08bf99f4182dafcb94d94104fcdfae693d303eb7` into integration `d0e7331525fec046fbe9361906fa66367ae583c6`.
- The new scoped current-operation exclusion, CHECK/inventory, fixed-code mapping and all four immediate/distinct/consent/no-effect witnesses failed their intended assertions. Existing policy and UNKNOWN assertions passed. Native filtered evidence is committed under `output/oa-2/rvfix305/review-red-job.log`; receipts are in `output/oa-2/red.log` and `output/oa-2/rvfix305/red.log`.
- Foundation `40895129674` / job `123522034094` passed on the RED head; it does not execute this document suite.
- Current implementation GREEN and independent delta review: PENDING. Exact implementation head and native Gates/Foundation WAIT_RUN IDs will be recorded in a PR comment after push. Historical green is not current acceptance.
- E3 applies to the observed RED document gate only. Own assessment E0; no SQL runtime acceptance. No local tests or servers.

## Scope and handoff

Current implementation batch: `output/oa-2/SQL-REQUEST.md`, delivery/RED receipts under `output/oa-2/**`. No source or test edits. Fetched/merged `origin/r3/integration@d0e7331525fec046fbe9361906fa66367ae583c6` first (already current); browser ownership regenerated last before push.

Outside-card file in this PR: generated `output/ci-select-backend-browser/browser-ownership.json`, required by CI ownership policy and explicitly pre-authorized; no hand edits. No other out-of-card edits by this batch. Integrator final review remains pending.

P2 for issue: reminder identity NULL gap remains open. Integrator schema follow-up must require both keys IS NOT NULL and separate omitted-key REAL_PG negatives before adoption. Existing follow-up: https://github.com/humaux/live_commerce_architecture_v1/pull/305#issuecomment-6498353837.

NOT_RUN: current-head GREEN; REAL_PG C3/C4/ACL/lock barriers/DB mutations; scheduler/cancel/rotation/copy/reminder composition; provider I/O; OA-3; SANDBOX/LIVE; production; independent delta review. Document assertions cannot prove executable SQL behavior.

Wake-up owner: integrator relay on native CI completion/failure/cancel or >3h timeout. No local background process, merge, auto-merge, workflow rerun or evidence-only push. Humaux/coord unavailable this relay: no store/canvas update claimed. Pre-existing untracked work retained.

Rollback: revert this documentation implementation commit on `unit/oa-2`; no DB effect exists.

🤖 Generated with Codex (dispatched by Claude Code integrator).
