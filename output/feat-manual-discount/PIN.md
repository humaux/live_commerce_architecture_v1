# FEAT-MANUAL-DISCOUNT — dispatcher PIN handoff

Date: 2026-11-01. PR #394 into `r3/integration`; branch `unit/feat-manual-discount`.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/feat-manual-discount`.
Role: Codex dispatched implementation worker; actual model/reasoning settings are not exposed. No delegation.
Humaux discovery returned no callable memory/coord/canvas tools; no memory store or canvas update is claimed.
No new task claim: this is the integrator's already-dispatched single PIN card.

## Authority and exact scope

Read AGENTS.md, 架构.md §§0,1,2,26,27, task DAG, `docs/engineering/contractdrift.md`, the OC-09b candidate/review
precedent, `contracts/cart-quote-v1.md:189-209`, and the current buyerhttp source diff.
No new implementation, script, test, gate or design was created; no additional skill is needed for this pin-only card.

Rollback reference: pre-pin HEAD `3a4e5f724edf9302766258f22a75d1dc8227758f`, pin blob at that HEAD;
integrator can revert the dedicated PIN commit if needed. No rollback was executed. Existing dirty FIX4.md
and untracked prior evidence were left untouched and will not be staged.

`git fetch origin r3/integration` and `git merge --no-edit origin/r3/integration`: exit 0, Already up to date,
integration SHA `fde11c1310fce20b576fc98023bd3003c66c205b`. The base did not move buyerhttp.

Only `buyerhttp.Routes`'s hash and pending-review string change in `contracts/route-table-dispatchers.json`.
Its 82 functions and 151 declarations have no additions/removals and remain unchanged/sorted. All other
entries, tests, `go_extract_test.go` pins, baseline, routes, source files and migrations are untouched this round.
The closure diff, required rationale, file:line and URL/method/auth/tenant-scope statement are in
`output/buyer-route-table/manual-discount-pin-candidate.md`. Only the integrator flips review status after
the PR's independent reviewer reads the entire closure and approves.

## RED and static evidence

Accepted original tests-only RED at `fc28de173547886f74900c5caf4ab1f7a675b451` remains in `red.log`:
Foundation **41093578253 / 124193423117** (thumbnail, BFF discount grammar, response parser assertions);
Gates **41093578464 / 124197225702,124197225711,124197225869,124197225752,124197225919**.
The PIN card explicitly forbids test changes; no redundant tests-only push or mutation was made.

Pre-pin E3 negative evidence, verified directly from GitHub at source HEAD `3a4e5f724edf9302766258f22a75d1dc8227758f`:

| Run / job | Failure |
| --- | --- |
| Foundation 41275855282 / 124757772556 | `TestGoRepositoryInventory`: missing buyer table rows after dispatcher freeze refusal. |
| Gates 41275855308 / plan 124757772578 | `buyerhttp.Routes a8b1feb94a2439eb02b09a2d42e10c0e273abacfd53dc31708725147d9a2590a want bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a`. |
| Gates 41275855308 / required 124758639722 | Downstream required aggregation fails after plan; feature jobs skipped. |

E1 checks: `node scripts/dev/derive-browser-ownership.mjs` exit 0 (55 modes, no artifact diff);
`node scripts/dev/derive-browser-ownership.mjs --check` exit 0;
`python3 scripts/check_packet.py` exit 0 (`PASS_PACKET_STRUCTURE_ONLY`, not runtime acceptance).
The required generator/readback is explicitly authorized by this PIN card. No local scanner, runtime tests,
model tests, server, database, browser, fixture, port or container was started.

Candidate pin file SHA-256: `081463c1e82f8dfb2fc9571df7f10c0f4ca3c6c41826b1f4c0f228b5878c0b29`.
Buyer projection SHA-256: `68d5cd2348fcdacfcec0314fac5698272041cdb17b0511d6030543a09b51e6e1`.
No source dependency changed, so no dependency-map update is required.

## Handoff and remaining verification

Author acceptance is E0; static structure E1; pre-pin CI failure is E3 negative. Candidate GREEN/E3 and
whole-closure independent approval/E4 are pending. New head/run/job IDs will be recorded in a PR comment
and the final handoff after the single implementation push; no evidence-only follow-up commit.
CI completion notifies the integrator, who relays the result to a fresh session. Do not silently poll for hours.
Changes outside this PIN card's paths: none. Generated browser ownership was regenerated but is unchanged.
NOT_RUN: all local runtime/unit/integration/browser/model tests and servers; local contractdrift scanner;
candidate CI pending push; independent whole-closure review; SANDBOX/LIVE and production deployment.

STATUS: PIN_CANDIDATE_CI_AND_INDEPENDENT_REVIEW_PENDING
