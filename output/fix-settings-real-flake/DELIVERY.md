<!-- Purpose: Explain and evidence the settings wizard's two real-user readiness races.
Depends on: merchant-settings-wizard-v1 W01/W03, REAL_PG identity harness, CI ident-10/ident-3 artifacts.
Used by: integrator pre-review and PR opening; not production/provider acceptance. -->
# fix-settings-real-flake

- Branch: `unit/fix-settings-real-flake`; base `424f17ccd3fd8db00923d91d3d23ca8bfafc249f` (fetched origin/r3/integration; already current).
- Source: **4b231688fecf0ff86f2c86f019eda95ffb74fc45**. Later evidence-only commit does not change the tested product/spec.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/fix-settings-real-flake`. Author: Codex; exact model/effort identifiers not exposed. No delegated code writers.
- Scope: `SettingsWizard.tsx`, `settings-real.spec.ts`, this evidence directory. No API/SQL/contract change, no push/deploy.

## Root causes (product, not a slow locator)

Both supplied CI snapshots show **Configuration changed. Reload before saving a corrected version**. In ident-10 the policy PUT never occurred; in ident-3 policy PUT succeeded but service PUT never occurred. The later missing form/status locator is a symptom of a synchronous local CAS refusal, not a response requiring a larger timeout.

1. **P1 — Save was actionable before the baseline read finished.** Editing the service code clears observations; edits mark their version `-1` until GET resolves. Save checked general session readiness, not target-baseline readiness. A fast merchant could click while observation was null/-1; `baseline()` then falsely reported a concurrent modification.
2. **P1 — A policy PUT receipt discarded a concurrent service GET.** The callback spread its render-time `draft`. If the service GET applied version 0 during that PUT, the receipt restored the older null observation. Service Save then produced the same false conflict. Waiting for an HTTP response alone would not address this state overwrite.

The product now disables each versioned Save until its target observation exists with version >=0 (policy/service/payment method). Existing localized loading text explains the manual-delivery wait. The **real stale-version equality/CAS check stays intact**. Normal and journal-replay policy/service receipts merge the current draft, preserving sibling observations. No automatic command retry or new command key is introduced.

## Red → green and assertion integrity

- Unmodified product/spec: `baseline-identity.log`, **exit 0**, three Go gates. Together with the supplied CI failures, this is the intermittent baseline; it is not a measured failure-rate estimate.
- New controlled regression on original product: `readiness-red.log`, **exit 1**. Detailed failures in `red/playwright.log` and `red/error-context.md`, screenshot `red/test-failed-1.png`.
- The added test holds the **actual PG-backed GET and PUT responses**, not substitute response bodies. It first proves Save is disabled before policy GET completes. It then holds the policy PUT receipt, releases service GET and reads only the persisted synthetic draft's version to prove it was applied, before releasing the receipt. Original status/persistence assertions then expose the stale overwrite.
- Red records **both** failures: expected disabled/received enabled, followed by the original `settings-status` missing assertion. The new soft assertion is still test-failing (exit 1) and permits observing both defects in one run.
- Product fix: `readiness-green.log`, **exit 0**. Five further consecutive load runs are below. `green-playwright.log` is the final iteration's spec result.
- No original assertion was removed/changed; only the Route type import and extra test steps were added. No DOM/storage mutation, force-click, sleep, retry, timeout increase or fake PG receipt. Existing action/expect limits remain 10 s, spec limit 90 s, Go harness 100 s, Playwright retries 0.

## Commands / exits

| Command | Exit / result | Evidence |
| --- | --- | --- |
| `pnpm install --offline --frozen-lockfile` | 0, lockfile unchanged | `install.log` |
| `LC_TEST_LOCK_WAIT=14400 nice -n 10 bash scripts/dev/test-local.sh --browser-identity` — original baseline | 0, 3 Go PASS | `baseline-identity.log` |
| Same mode, new regression / original product | 1, settings RED; other 2 Go gates PASS | `readiness-red.log` |
| Same mode, fixed product | 0, 3 Go PASS | `readiness-green.log` |
| `bash output/fix-settings-real-flake/repeat-under-load.sh` | **0, rounds 1–5 all exit 0; 15 Go PASS** | `repeat-under-load.log`, `repeat-1.log` … `repeat-5.log` |
| `bash scripts/dev/test-node.sh` | **0, 618 PASS / 0 FAIL**, summed runners | `node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc.log` (empty success log) |
| `LC_HEADER_BASE=424f17cc bash scripts/dev/check-gates.sh` | **0, 78 modes** | `check-gates.log` |
| `git diff --check` | 0 | author command receipt |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | 0 | `ci-plan.json`, `ci-plan.log` |

The repeat driver is a **serial stability experiment, not failed-test retry**: it stops on any nonzero exit. Four bounded CPU workers run at **nice 19**, gates at **nice 10**, with the existing machine-wide PG lock. At sampled boundaries the machine load reached **17.15**. This Mac's `node` command is a Python wrapper, so its parent shows 0% CPU; `load-child-evidence.log` records the actual four node children at approximately **87–89% CPU each**, nice 19. The script's exit cleanup also terminated those children; their exact PIDs were verified absent afterward. No other agent's process was stopped. Product/spec stayed unchanged throughout all runs.

### Source SHA-256

- `apps/admin/components/SettingsWizard.tsx`: `6a8f7069ea3a0c9df8834188129409e987c15c3416e8d02d13f47300b5f37d54`
- `tests/admin/settings-real.spec.ts`: `7410b54959aa9243f59fa5026ce64223e3738dee34aaa592f4e461e5a3c7dd04`

## Acceptance / CI gates / remaining work

- **E3, REAL_PG + browser + signed MOCK IdP**: both product races reproduce red before the fix; fixed source passes the whole identity mode six times, including five under CPU load. Existing three-locale/responsive, persistence, stale-CAS and safe-uncertainty flow remains in the spec.
- Natural CI evidence supplied by integrator: job `112908142127` in `ident-10` (fill line 490), job `112909344875` in `ident-3` (status line 500); local originals under the supplied scratchpad. Local deterministic red is independently retained here.
- **CI gates:** required planner output in `ci-plan.json`; targeted mode `--browser-identity`. Other conservative all-browser selections, full foundation, global sweep/visual lint are **NOT_RUN locally** for this small unit. Integrator's required PR matrix remains pending.
- Optional R04 input runner is NOT_RUN (`COMMERCE_R04_LIVEKIT_BINARY` unset), excluded from Node pass count. Live identity/provider/production behavior is NOT_RUN.
- Independent pre-review and PR/CI acceptance pending; no E4 or merge claim. Integrator pushes and opens PR. This is not authorization to deploy.
- No owned gate or CPU process remains; task-owned PG fixtures are cleaned by each mode. Raw failure evidence is intentionally retained. Code index accepted both files (62 entities).
