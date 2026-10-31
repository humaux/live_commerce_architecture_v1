# Lane report: RVFIX2-366 relay 1

| Failing job / finding | Class | What changed |
|---|---|---|
| gate (bundle:03) `--browser-meta-ads` TestBrowserMetaAds (head eb7527d04, run 41251144024, job 124685707869); receipts/required are consequences | a (expected RED on the new outsideAdvanced assertions, tests/admin/ads.spec.ts:412,413,453,646) | Recorded in output/parity-growth/red.log. Implemented in apps/admin/components/AdsDraft.tsx: ads-actions block, end-confirm and hints moved out of `ads-advanced`; new `ads-refusal` (latest Meta error_user_msg) for FAILED/REJECTED. Ads.tsx unchanged. |

Notes: CI log carries no node stderr, so the class rests on the failing mode being only meta-ads and the specs asserting exactly this. Nothing run locally (owner rule). Report: output/parity-growth/RVFIX2.md.

WAIT_RUN

STATUS: DONE
