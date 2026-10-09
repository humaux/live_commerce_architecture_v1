# PR2 round973 Studio: first failure located, product root not proven

Status: **UNRESOLVED**, not claimed fixed. No Studio product or acceptance-test change.

CI run37713621244 job113105067253 first fails `studio-ui.spec.ts:198`: native picker click → ArrowRight → Enter leaves `2030-01-01T08:00` unchanged. Initial create POST, title PATCH and reload reads succeeded with200; no page error. Main test has not yet armed its lost-ACK fault or started the worker. Harness key/effect/schedule/terminal assertions run after the whole suite and consequently report missing downstream effects. The `authority=0` counter means no forbidden forwarded authority was seen. These counters do not prove changed retry keys or migration corruption.

Evidence: `studio/playwright/studio-ui-20261008T013714.154258400/results/studio-ui-STU04-signed-Stu-46f44-Go-PG-and-local-MOCK-worker/trace.zip`, SHA2564235ffe6bf17e26333ee64c8aff33774ca2a81a1f1b8dce8032e2d325f47caec. Actual click(1103.43,400.1), picker rectangle(1082.4375,378.109375,42,44), viewport1586x992. The anchor is not beyond1280. Original artifacts do not show the native popup rectangle, physical screen/window offsets, native key routing, or setter call stacks.

Studio/component/CSS/spec/harness comparisons are in `studio-unchanged-source.json`; these files are identical to18f82636. Parcel styles inspected have no new Studio/generic-input rules. This narrows suspects but is not proof that every timing effect of the PR is impossible. The old October5 readiness explanation was not adopted.

Local full gate on untouched973: `bash scripts/dev/test-local.sh --browser-studio-ui`, exit0 (67.425s), recorded in `studio-baseline.{json,log}`. This is a baseline PASS, not a repaired Studio regression.

Bounded native-control experiments (MOCK HTML shape, not the complete Studio application):
- macOS ARM headed Chromium, no tracing: existing explicit showPicker15/15 changed, preventDefault+showPicker15/15, native-only0/15.
- Same with tracing:15/15,15/15,0/15. Existing handler therefore30/30 on these Mac controls.
- Pinned Linux amd64 Playwright1.63 under local ARM Rosetta: NOT_RUN; xvfb-run remained at startup before Node/Chromium. Only the owned labelled container was removed (wrapper exit137).
- Native Linux ARM Playwright1.63: existing handler15/15, preventDefault14/15, native-only0/15. The isolated xvfb-run PID1 startup wait was released with SIGUSR1 only after its X socket existed; Node/Chromium then ran. This is not GitHub amd64 evidence. Container was removed automatically; command exit0 records completion, not45 successful selections.

These observations do not support adding preventDefault, changing Studio readiness, altering date expectations, or relaxing waits. The native-only negative control shows the overlay still needs explicit activation. No unrelated handler patch was made.

To establish the remaining root, a reproduced failure needs native popup/keyboard routing and input/change/value-write chronology on the actual Linux test environment, alongside the existing response trace. A native value then overwritten by React would identify a controlled-state bug; no input/change with keys delivered outside the popup would identify a native activation/focus boundary. Neither was captured in the failed CI artifact. Local green does not close this root-cause gap.
