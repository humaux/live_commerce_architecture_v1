# LANE-REPORT — RVFIX-366 relay 3 (head 1229cc82d, run 41242263861)

| Failing job / finding | Class | What changed |
|---|---|---|
| `gate (--browser-webkit)`: only member `password-auth` red (`TestBrowserPasswordAuth`, test-local.sh:956); buyer/order/payment/merchant-buyer/cvs/claim-checkout green | c: not caused by this PR. The diff touches no auth/password file, and every promotions, admin, click-sweep and visual-lint job on the same head is green. The failing assertion is only in the jsonl artifact, not the CI log. | None. Merged origin/r3/integration (1 commit, 4501b409c); `derive-browser-ownership --check` is clean after the merge. |
| `receipts`, `required` | c: derived. The `--browser-webkit` receipt FAIL is the single problem. | None. |

Fix attempts used: 0 of 2. If webkit `password-auth` fails again on the new head, STOP: the integrator must read its jsonl artifact, which this lane cannot reach.

WAIT_RUN: re-run `gate (--browser-webkit)` on the new head.

STATUS: DONE
