# PARITY-ENTRY-FIX2 relay 3 report

Head 3be83d29 had no CI (merge CONFLICT). Failing run 41170112825 is on an older head (aaef95b97 lineage); the fixes made since
(6509e2919 label hint + order timestamp, 3be83d29 BFF cache-control/reset text) have never been run by CI. No new evidence to classify.

| failing job/test | class | what changed |
|---|---|---|
| merge conflict with origin/r3/integration | n/a | Merged; only conflict was generated `output/ci-select-backend-browser/browser-ownership.json` (2c9bbc753), regenerated with derive-browser-ownership.mjs; `--check` printed no drift |
| gate (bundle:06) TestBrowserInboxUIRealChain / --browser-inbox | not re-evaluated | no new CI on current head; earlier directed fixes a74dfa7c4 (desktop inbox navigation) already in branch |
| gate (bundle:08) TestBrowserPasswordAuth / --browser-password-auth | not re-evaluated | earlier fixes 6509e2919/3be83d29 already in branch (`PasswordAuth.tsx` hint outside label, `parity-entry-dashboard.ts` timestamps) |

No product/test changes this relay. Tests were not run (owner rule). Integrator: push merge head 2c9bbc753, then WAIT_RUN on the new CI run.

STATUS: DONE
