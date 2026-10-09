<!-- Purpose: PR18 Qwen addendum hardening of host digest tracking and bounded settle, with causal gate evidence.
Depends on: 07cc90e5 and deleted-comment packet addendum; actual inbox-review host and Node child_process isolation.
Used by: integrator K3 pre-review and PR18 CI; author-local E3, not independent acceptance. -->
# PR18 host hardening addendum

- Branch `unit/lc-u2a-comment-stream`; base **`56364989`**, retains deleted-comment fix97afbe2c on top of1c43bfe4.
- Tested source **`d8e1c29017db0cc383eb590f50134f9119b1e814`**; final evidence commit changes no test/product source.
- Codex-1, no delegates; exact runtime model/effort identifier unavailable. Two TEST files changed, no product/Go/SQL/registry change. No push or thread resolution. W3-U3 paused.

## Changes

The WebCrypto wrapper now decrements `pendingDigests` when its captured native digest throws synchronously, then rethrows the same exception. Promise completion/rejection still decrements through the existing finally path.

`Host.settle()` now asserts `pendingDigests === 0` after its original bounded loop, with the requested message `digests quiesced within bounded rounds`. Round/turn limits were not increased.

Two appended isolation cases import the actual host in separate Node processes after overriding only the native digest boundary. One native implementation throws synchronously; the test requires the identical error and a subsequent healthy settle. The other holds a Promise through the bound; the test requires ERR_ASSERTION with actual1/expected0 and the requested reason, then releases the Promise and verifies recovery. No counter/host stand-in or exported inspection seam.

`original-suite-parity.log` proves the original suite body remains byte-identical after its dependency header: **78 assertions and their order unchanged**. New cases are appended, not interleaved with existing cases. Child processes have a10s outer failure bound; successful/failing digest promises are cleaned up in the child.

## Red → green

| Negative control | Exit | Outcome |
|---|---:|---|
| Remove synchronous catch/decrement, retain bounded assertion | 1 | sync-throw FAIL; never-settles PASS (`red-sync-final.log`) |
| Remove bounded assertion, retain sync handling | 1 | never-settles FAIL with missing expected rejection; sync-throw PASS (`red-bound-final.log`) |
| Both fixes restored | 0 | 2/2 PASS (`green.log`) |

Initial exploration logs (`red-bound.log`, `red-sync.log`) remain as history. The latter exposed that Node AssertionError's message includes a diff, so the final test pins assertion code/actual/expected plus a reason regex, rather than treating the entire message as a bare string. Final negative controls above each fail only their intended case.

## Final gates on d8e1c290

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='PR18 host digest hardening' tests/admin/inbox-review.test.ts` | 0 | `green.log`,2/2 |
| `bash scripts/dev/test-node.sh` | 0 | `node.log`,**1200 tests /0 failures** |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`,82 documented modes |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `browser.log`,**23/23**,Go325.29s |

Browser artifact: `output/playwright/live-console-1410857524/`; safe result retained in `browser-results.txt`. This is MOCK Graph + real Next/Go/PG, not LIVE Meta. Source remained fixed during runs; browser/PG runs strictly serial. SHA256 binds both changed files.

## Combined packet handoff

The deleted-comment client fix remains in this branch; its6 Node regressions and qualified evidence are in `../pr18-deletion/DELIVERY.md`. Its deletion-specific browser case remains **NOT_RUN** because the real ring fixture only evicts by age/cap and cannot drive a same-epoch missing-id deletion. That backend follow-up is unchanged; this hardening does not establish LIVE deletion behavior.

E3 for the requested test-host changes and local gates. New independent K3/required PR CI, deletion-specific browser, LIVE/production and full foundation remain NOT_RUN. Unrelated R04 runner remains the explicit test-node skip because its binary env is unset. All owned commands exited. Tracked existing output evidence and other agents' untracked material were preserved. Commit and stop; integrator owns push and review replies. CI gate: `--browser-live-console` plus repository-selected required PR gates.
