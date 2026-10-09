# Existing stale-tab browser assertion: red to green

- RED: e27728c5 MOU r3, exit 1; original 10 orders cases passed. Actual shipment PUT returned 409 `in_parcel_group`; the real group refresh completed about 19 ms later. The OPEN group hint replaces the entire form containing `shipment-problem`.
- Trace SHA256: `252814e77cbd5fa6c01306f9854ddbb083783227cc84f8b7d3ff7204634fc700`. Trace under `output/playwright/merchant-orders-c-browser/20261007T234622.801858000/results-parcel/`.
- Change: hold only page A's actual post-refusal GET groups response, assert the real PUT 409/code and unchanged error/no-record assertions, then release the unchanged response and assert the unchanged group/Open/badge/hint/form-absent outcomes. Bounded waits and finally release. No product change or CVS wait alteration.
- GREEN: MOU r4 exit 0; 10 original orders cases plus complete parcel scenario (including six authoritative read denials). Source test SHA256 is recorded in `codex4-mou-r4.json`.
