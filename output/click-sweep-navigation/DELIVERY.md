# D4 click observer causal regression — RED stage

task_id: 53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-sweep-navigation
base_commit: c52b65283a5ac738d0c527a464d0cccc9523b1f4
branch: codex/click-sweep-navigation
role: test_worker; dispatched model/effort: gpt-6.1-sol/high (runtime exact identity not independently exposed).

Tests only: existing sweepControl is exercised by synthetic loopback Chromium pages. Browser gate invokes the existing Node test file under LC_SWEEP_NAVIGATION_CANARY=1 before PG/Next setup; ordinary Foundation tests remain browser-free. Three real clicks: inert button, preventDefault submit, native GET same-URL submit. No production file, timeout, retries, weights, exemptions or existing assertion changed. Generated browser ownership follows the existing generator.

Causal RED requires both negatives remain no-effect FAIL with one document/timeOrigin unchanged, while native GET requests exactly two documents, preserves exact URL, changes performance.timeOrigin, and fails the navigation-effect expectation. Environment failure is not causal RED. All cases print SWEEP_NAVIGATION_CANARY JSON to navigation-canary.log before verdict assertions. The existing helper remains unchanged until verified RED.

Commands actually executed locally (exit 0): node --check tests/ui/click-sweep-lib.test.mjs; gofmt -w tests/foundation/browser_click_sweep_test.go; node scripts/dev/derive-browser-ownership.mjs; node scripts/dev/derive-browser-ownership.mjs --check; git diff --check. Runtime/browser tests and servers NOT_RUN locally by owner instruction. Evidence retention checked against origin/r3/integration before push.

Evidence: forthcoming draft PR CI native logs/artifacts, click-sweep/*/navigation-canary.log; no raw artifacts copied. Browser test timeout 45s, Go child bound 1 minute, existing workflow timeout unchanged. GitHub native PR/check notifications hand back to integrator; no local background process.

UNKNOWN: original PR440 three failed clicks actually reloaded; this canary diagnoses observer causality only. NOT_RUN: causal RED, repair/GREEN, original-row navigation tracing, independent acceptance, LIVE. Current author claim E1 static structure only. No merge/automerge/deploy authorized.

## FIX1 — verified causal RED and document-origin comparison

RED SHA: ac0aa7c860c9b8d6965a0decdfbc5af026f5425e. Gates41294631534/job124813553322 (click-sweep6/10), artifact13078432836, zip SHA256 9d0275dfd439dceb3f5049fdd1d312db54f1289d37160b177a0fb93b7f515193. Entry playwright/run.PyuGOfrN/click-sweep/20261101T095213.862701919/navigation-canary.log: 15PASS1FAIL. Native same-url: two document requests, exact unchanged URL, performance.timeOrigin1793526742882.7→1793526744034.8, helper FAIL/no-effect; noop: one document,1793526734398 unchanged,FAIL/no-effect; preventDefault: one document,1793526738665.8 unchanged,FAIL/no-effect. Failure is the expected pass versus actual fail assertion, not environment setup. Archive parsed entirely in memory, not retained.

Fix: successful pageState now carries performance.timeOrigin; navigation needs unequal finite positive before/after document origins or the existing URL change. A failed state read alone no longer earns navigation success. Same-document no-op controls retain no-effect FAIL. Two additional read-boundary negative controls remove/set a non-finite origin without mutating the browser page, and must still fail no-effect. Every row retains before/after origins and the computed document-change flag. All existing time windows, errors, classifications and actionability assertions remain. Synthetic fixture uses the browser-provided performance value; deliberate application tampering with that getter is not independently established by this signal.

FIX1 static commands: node --check helper and test; browser ownership generate/--check; git diff --check; evidence-retention --base origin/r3/integration. Runtime only on next exact-head GitHub CI. Original PR440 three failures remain unattributed until those rows are rerun with the added evidence. Independent review/GREEN/LIVE remain NOT_RUN.
