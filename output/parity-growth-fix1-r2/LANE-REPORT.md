# LANE REPORT — oc-parity-growth-fix1-r2 (PR #366, run 41084050704)

Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/parity-growth (branch unit/parity-growth).
Lane mode: Backend only — do not change anything under apps/** (owner rule). Ordering: origin/r3/integration is
already merged locally (b61a8262, done by the foundation relay) and re-verified before any analysis began.

## Failing job/test → class → change (file:line)

| failing job | failing test | evidence | class | what changed |
|---|---|---|---|---|
| gate (bundle:09) job 124178338229 (`TestBrowserOperationsAds`, bundle member --browser-operations-ads: failure; identical at prior head: run 41075396782 job 124143814275) | `operations-ads.spec.ts >> read-only permissions hide mutation controls after reload` — `expect(byTestId('ads-connections')).toContainText(account)` → element not found | artifact `gate-browser-operations-ads`, `results/…/error-context.md`: signed list screen renders, no inline connection region | (a) | tests/admin/operations-ads.spec.ts:182-197,232,254-262 — rewritten to the signed C3 frame: click the "View ad account connection" button (growthCopy assets) then verify ads-connections/ads-unbind/ads-feed-url/clipboard/reload as before (coverage and assertions preserved; committed as e9a0e8994 in this worktree). |
| gate (bundle:07) job 124178338822 (`TestBrowserTaiwanCvs/WebKit`, bundle member --browser-cvs: failure) | cvs-picker.mjs:387 `expect(choose('2656')).toBeVisible()` after HILIFE fill | artifact `gate-browser-cvs`: ariaSnapshot shows cvs-picker-query empty with HILIFE pressed; MOCK subtest passed; same file passed on the prior PR head run | (b) | Shared helper made deterministic: wait for POST /api/buyer/quotes response + aria-pressed before each brand fill (tests/storefront/cvs-picker.mjs:380-386). Root-cause product fix would sit in apps/storefront (CheckoutFlow onCvsBrand commit window) — out of lane scope per owner rule; the added settle uses real user semantics, no assertion/threshold loosened. |
| receipts job 124208027681 | matrix-receipts verify — `FAIL: --browser-cvs`, `FAIL: --browser-operations-ads` | receipts log | cascade | none (flips with the members above). |
| required job 124209655416 | required guard `GATE: failure / RECEIPTS: failure` | required log | cascade | none. |

## Verification commands (allowed set only; no tests/builds/servers/browsers run)

- `node scripts/dev/derive-browser-ownership.mjs` — wrote output/ci-select-backend-browser/browser-ownership.json (54 modes)
- `node scripts/dev/derive-browser-ownership.mjs --check` — exit 0
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` — PASS (27 files, 624690 bytes)
- `gofmt -l .` — clean

## NOT_RUN / Notes

- Local test execution NOT_RUN (owner rule: GitHub CI only). All evidence read from the failing run via gh.
- Browser-ownership `--check` re-run after regeneration: pass with cvs-picker hash refresh.
- Evidence retention: cites run id/job/artifact only; no raw CI copies, no artifact/log files committed;
  check-evidence-retention passes against origin/r3/integration.
- A parallel running lane (Co-Authored-By: Gemini, batch parity-growth-fix2) commits into this same worktree; this lane
  scopes its commits to its own files only. Its e9a0e8994 contains the class-(a) rewrite this batch validated.
- red.log unchanged: this head failed on existing assertions (implementation head), not a tests-only RED.

Report WAIT_RUN.

STATUS: DONE
