# LANE-REPORT PARITY-LIVE-FIX2

| Finding | Class | Change |
|---|---|---|
| TestBrowserInboxUIRealChain / bundle member --browser-inbox (job 124266257034) | (c) known base race in enterMessages; fix a122b3cd6 rides PR #356; PR touches no inbox files | none; documented in output/parity-live/FIX2.md |
| receipts / required | follow from the above | none |
| evidence-retention gate: 24 raw CI artifact copies tracked | (b) | `git rm --cached` of output/PARITY-LIVE/{fix1-relay1,red-diagnostics}/*.gz etc. (commit "untrack raw CI artifact copies"); check PASS |
| merge origin/r3/integration | done | merge commit 816200fe8 |

Per-case Playwright evidence was not readable in this lane (artifact download not permitted); class (c) rests on the empty inbox diff and the failure signature in the integrator note.

STATUS: DONE
