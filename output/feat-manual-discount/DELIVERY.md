# FEAT-MANUAL-DISCOUNT FIX1 evidence manifest

2026-11-01. See FIX1.md for classification, scope, static checks and limitations.
Author acceptance E0; static E1; prior CI failure E3 negative; new-head GREEN and E4 pending.
No new UI design or seller/buyer copy changed; UI design-process/locale additions do not apply to this harness-only fix.

| Evidence | GitHub origin | SHA-256 |
| --- | --- | --- |
| WebKit original artifact archive | run 41277742564 / job 124763804297 / gate-browser-webkit / artifact 13072148027 | afd8aa453119a39336d4c00ec3cf54fe833ee78d52939afa8309ec48e9e6c7a0 (GitHub digest) |
| Original browser.log | same artifact, playwright/buyer-order-2036392307/browser.log | e04b1307d2fbb6168eeafa7237a5d9ef7f871512015476430342ca9a0415e773 |
| Original failure screenshot | same artifact, playwright/buyer-order-2036392307/failure-7.png | 0aaf49c201b1e59f7171d792a1b570a0aa715feeea00419375b12ee4d68ccbeb |

Original failed jobs are re-readable with gh run view --job <id> --log-failed.
Artifact is re-downloadable with gh run download 41277742564 -n gate-browser-webkit.
Raw downloaded evidence remains local/untracked; no artifact, raw job log, capture or archive is committed.
Final new-head run/job receipts belong in a PR comment, not an evidence-only commit.
