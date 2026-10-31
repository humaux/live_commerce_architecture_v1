# RVFIX2-403 RELAY 1 lane report

| failing job / finding | class | change |
|---|---|---|
| gate (bundle:06) on e93dda8d (RED push, run 41227154302) | a (expected RED on the new duplicateMarketAcceptance assertions) | apps/admin/components/SettingsWizard.tsx ~:995: the POST markets success callback now calls `setMarkets` with the created tw row before the PUT is sent; draft.marketID still changes only in the PUT success callback |
| receipts / required | c (downstream of the bundle:06 red) | none |
| retryPending markets branch (:807) | already inserts the replayed market into `markets` | no change needed |

Local tests not run (owner rule). RED run id 41227154302; GREEN run id pending next CI.

WAIT_RUN: next CI run on the pushed head

STATUS: DONE
