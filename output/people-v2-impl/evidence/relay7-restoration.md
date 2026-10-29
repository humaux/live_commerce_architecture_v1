# CONT289 restoration — CI pending

2026-10-29. Assigned implementation continuation, no delegation. Runtime model/effort
metadata is not exposed. Worktree `people-v2-ui`, branch `unit/people-v2-ui`, target
`r3/integration`. Starting local HEAD `f63884bfc25dd4c2435e976753b9ffbcf4e2c5e3`;
required fetch/merge exited 0, already up to date. Existing delivery/evidence edits retained.

## Current-head evidence and independent review P1

GitHub PR metadata and `gh pr checks 289` identify remote head
`d409e4094b797aa86987f49923f3e64ce6bf2f99`. Gates `40803107511` completed failure,
tested synthetic merge `23f0dac876c0296755cfee8522ec8dce4c0245c6`.
All four failed jobs were read using `gh run view --log-failed --job`:

| Job | Root cause / disposition |
| --- | --- |
| bundle07 `123216904686` | Deliberate privacy negative controls; restore safeguards after causal assertion RED. |
| bundle08 `123216904501` | OC signup helper races the post-verification redirect with createStore navigation. |
| receipts `123235265909` | Downstream FAIL receipts: customers-billing, migration-import, password-auth. |
| required `123235420197` | Downstream gate and receipts failure. |

Fresh `gh run download` retrieved all three failed browser artifacts. Original Playwright
logs were independently read and byte-compared with the retained same-run originals.
`relay7-red-assertions.log` contains compact failure excerpts (trailing whitespace trimmed) and source hashes:

- Export: `people-v2.spec.ts:174`, review button absent before any authorized confirmation.
- Erasure: `people-v2.spec.ts:132`, correct name without scope checkbox incorrectly enables submit.
- Independent uncertainty: `people-v2.spec.ts:234`, both RETURN and RELOAD leave export enabled.
- Migration import: `import-wizard.spec.ts:333`, the same intentional scope mutation.

This is E3 causal privacy RED and addresses the missing negative control identified in
the review of `94af3eb58`. That head's historical GREEN does not accept this restoration.
Foundation `40803107529` / `123215600971` passed only at the negative-control head.

## Minimal restoration and OC setup fix

Reverse only `cont289-negative-control.patch`: export opens review; erasure requires the
exact name AND checked scope in both control and submit handler; unresolved outcome fences
both commands. Restored CustomerDetail source is byte-identical to reviewed `94af3eb58`.
No PEOPLE assertion or fixture changes; four independent uncertainty cases remain intact.

OC raw artifact fails `createStore` at `reauth.spec.ts:120` with ERR_ABORTED; its snapshot
already shows onboarding. `signUp` previously awaited only the verify response, while
`PasswordAuth.sendVerify` calls location.assign after parsing it. All signUp callers next
create a store. The shared helper now waits for the expected locale landing URL (including
the default load state) before returning. No retry, sleep, timeout or assertion is weakened;
the invitation replay count assertion remains unchanged. This is OC fix attempt 1.
Runtime causality and restored behavior still require this push's own CI.

Scope authority: the CURRENT card's SCOPE PRE-APPROVAL explicitly includes same-admin
root-cause repairs. Earlier relay output STOP interpretations are historical, not authority.
Additional outside-PLAN path: `tests/admin/oc/reauth.spec.ts`; no auth product, backend,
API, migration, dependency, lockfile or contract change. Browser ownership regenerated;
depmap regenerated with no diff. No new imports, so no component harness changes needed.

## Static receipts, acceptance gate and handoff

- `git apply --check --unidiff-zero -R .../cont289-negative-control.patch`: exit 0.
- `node scripts/dev/derive-browser-ownership.mjs`: exit 0, 53 modes.
- `bash scripts/dev/depmap.sh`: exit 0, no generated diff.
- `node --check tests/admin/oc/reauth.spec.ts`: exit 0.
- `node --check tests/admin/people-v2.spec.ts`: exit 0.
- `git diff --check`: exit 0 before commit.
- tsc NOT_RUN: this worktree has no compiler or installed admin dependencies.

Source structure only E1; author assessment E0. GREEN, real-click persistence acceptance,
and independent delta review remain pending the new head. Required gate: this PR's own
Gates + Foundation, preserving all existing assertions, then independent review of the
restoration/OC delta. One batched implementation push; final head/run IDs go in the PR
comment and handoff, not an evidence-only push. Integrator relays completion in a fresh
session; no polling beyond ten minutes, no merge/auto-merge or production action.

P2s remain for integrator issue batching: definite export refusal outside the active modal;
page-size count labeled as total. Per card, neither is changed in this continuation.
NOT_RUN: all local tests/servers/builds/browser, tsc, new-head GREEN, independent delta
acceptance, LIVE/production. Rollback: revert restoration commit; negative control must
never merge or deploy. Retain original failed CI artifacts; no other-task data is removed.
