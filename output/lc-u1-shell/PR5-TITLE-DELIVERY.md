<!-- Purpose: Deliver PR5 comment4206805918 title-state repair plus latest trunk/PR workflow merge, with scoped evidence.
Depends on: SessionCopy closed-form behavior, origin1aad42d0, command journal ownership and five local gates.
Used by: Integrator push to PR5 and automatic full required-set CI; not a deployment or full-foundation claim. -->
# LC-U1 PR5 — SessionCopy title sync

- Own branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- `git fetch origin` completed; merge **49dca3b8** includes latest **1aad42d0** (PR6 PNG pixel split/package docs/PR-triggered required gates and preceding PR4 fixes).
- Route allowlists, test-local modes and test-node suites retain both sides. Only `.github/workflows/gates.yml` conflicted this merge: preserve `LC_SWEEP_WORKERS=1` **and** trunk's dispatch-only `EXTRA_ENV`/new PR-triggered required set. No unresolved entries.
- PR https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/pull/5; integrator owns push/required CI/independent review/squash merge.
- Parent model not exposed; one inherited-model, explicit medium reasoning read-only explorer; no parallel writes/recursive delegation.
- Source **`5fb19555e936fb28de2d8bcbf3f7b107383c55ee`**. Final evidence-only commit preserves these hashes: SessionCopy `09dc630c5c4bd0d567d03f9bbed4ab986b1f6ccc44239d19e450bb2ffbb345bd`; hook/component tests `164284e47b03f00a2854b535e40d1222e8f0002c3849e181a7fc659e4858a306`.

## P2 root and minimum fix

SessionCopy's title state initializer ran once, while Studio keys the component by the same session id. A refreshed `draft.title` therefore did not change the closed form's next initial title.

Closed-only effect on **`[draft.title, open]`** now synchronizes the title. Open edits are not overwritten; cancel then reopen uses the latest source title. Preserve Studio's session-ID key and existing command owner: no remount/invalidate, no journal mutation or second write path. Submission still trims user input and reads the current draft version.

## Tests / evidence

The existing deterministic hook dispatcher loads the **actual SessionCopy TSX component**. Tests rerender same-ID props, invoke actual button/input/form handlers, and exercise existing command journal/retry logic. This is automated Node/component logic, not React DOM/browser/hydration proof.

| Command | Exit / result | Log in this directory |
| --- | --- | --- |
| `node --test --experimental-strip-types tests/admin/live-workspace-hooks.test.ts` pre-fix | **1**, stale original title: 9 PASS / 1 FAIL | `pr5-title-red.log` |
| Same command after repair | **0**, 10 PASS | `pr5-title-green.log` |
| `bash scripts/dev/test-node.sh` | **0**, **672 PASS / 0 FAIL**, summed runners | `pr5-title-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `pr5-title-tsc.log` |
| `LC_HEADER_BASE=1aad42d0 bash scripts/dev/check-gates.sh` | **0**, 79 modes/header ratchet | `pr5-title-check-gates.log` |
| `bash scripts/dev/check-pkgdocs.sh` | **0**, ok | `pr5-title-pkgdocs.log` |
| `bash scripts/dev/depmap.sh --check` | **0**, up to date | `pr5-title-depmap.log` |
| `git diff --check`; `git ls-files -u`; origin1aad ancestor | **0**, clean/no conflict entries | author receipts |
| Incumbent SessionCopy Impeccable detector | **0**, `[]` | `pr5-title-ui-detector.json` |

Regression also pins: typed open title survives source rename/version change; cancel/reopen follows latest source; submit uses trimmed user title + latest expected_version; after UNKNOWN, later prop/input changes leave durable key/body unchanged and explicit retry sends original bytes/key. No assertion/threshold relaxed. Independent read-only source review found no P0/P1; reviewer did not run runtime gates.

Evidence **E3 scoped local automated checks**. **NOT_RUN:** full foundation, new-source PR required set, browser/Linux/screenshots/provider SANDBOX/LIVE. Optional R04 input runner is NOT_RUN (`COMMERCE_R04_LIVEKIT_BINARY` unset), excluded from pass count.

## Handoff

Commit only, **no push**. Owner's explicit instruction overrides the general automatic PR workflow policy. All local checks finished; no owned background processes/PG/browser/container started or left running. Integrator pushes final SHA; the trunk PR workflow chooses and runs the **full required set automatically**, with no calibration/focus env on PR. New CI run id not yet available; integrator result notification resumes acceptance. W2-U2 remains queued until PR5 is accepted.
