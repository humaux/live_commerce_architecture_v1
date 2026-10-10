<!-- Purpose: K3 round-one P2 fixes, red/green evidence and unchanged backend blocker.
Depends on: actual label component/recorder tests, browser assertion parity and prior 992cff85 delivery.
Used by: integrator K3 review and the post-PR30 browser rerun; not an all-green/READY claim. -->
# W3-U3 K3 round 1 — two P2 fixes

- Tested source: `b245fef897837aaebefcd56ac90a1b475cd195bc`, based on `992cff85`.
- Scope: label denied-state recovery, spec observations, new Node test and its normal registry line. No Go/SQL/DTO/contract/library change.
- UI unchanged visually. Skills kept the existing transport and interface; only local error recovery changed.

## Fix / proof

1. **Ledger**: expected remains the contract description; actual is JSON captured from DOM/API state. Selection counts, matched label fields, paper/print-media geometry, request counts/statuses, native print invocation count and reload badges are measured, not copied. Outcomes follow those observed criteria. Failure teardown captures safe counts/state (or explicitly unavailable), never private text/ref/key. Lost-ACK observations are written before its original comparisons so the backend red records its actual unequal counts.
2. **Denied busy**: reset running/busy locally before calling onDenied for 401/403/404. The actual component test deliberately never remounts, retains the same AbortController, proves the button becomes enabled and a second request reaches transport; native print stays zero.

## Local commands / exit codes

| Command | Exit | Evidence |
|---|---:|---|
| Node label test before busy fix | 1 | `denied-red.log`: 3 denied cases fail with busy=true |
| Node label test after busy fix | 0 | `denied-green.log`: 9/9 (6 existing host +3 denied) |
| Node actual-source recorder before ledger fix | 1 | `ledger-red.log`: 2 recorder cases fail; 9 pass |
| `node --test --experimental-strip-types tests/admin/comment-label-print.test.ts` | 0 | `ledger-green.log`: 11/11, including 5 new regressions |
| `bash scripts/dev/test-node.sh` | 0 | `node.log`: 1319 executions /0 failures |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates.log`: 82 modes /1253 top-level Go inventory |
| Playwright spec collection `--list` | 0 | `spec-collection.log`: all 9 label cases, not browser acceptance |
| Spec tsc, using installed admin Node types | 0 | `spec-tsc.log`; root-only commands' missing type-path diagnostics retained separately |
| Browser assertion parity | 0 | `assertion-parity.json`: all 53 expect calls identical, same-key/count comparisons retained |

Spec typecheck command:
`pnpm exec tsc --noEmit --target ES2023 --module ESNext --moduleResolution Bundler --skipLibCheck --typeRoots ./apps/admin/node_modules/@types --types node tests/admin/comment-label-print.spec.ts`.

Only the network edge is fake in component tests; real inboxWrite/session fencing and label handlers execute.
The pure recorder test extracts its actual function with the TS AST, not a handwritten stand-in.

## Bounds / next

Evidence E3 for these Node regressions/static checks; no new browser acceptance claimed. Prior K3 PASS is on 992cff85, not this modified source. A bounded read-only author recheck found no introduced P0/P1; not independent K3.

Current browser ledger artifacts, updated screenshots, --browser-live-console/inbox/click-sweep/visual-lint and renewed PR CI: **NOT_RUN this round** (standing GitHub heavy-gate policy).
Old ledger rows that copied expected are historical, not independent observations for this batch.

**NOT_READY: lost-ACK same-key case stays BLOCKED(backend f4c26fcb / PR #30).** Its same-key/same-count assertions and real route.fetch/ACK-drop operations are unchanged; no skip, threshold relaxation or expected-failure annotation.
After PR #30 merges, merge origin/r3/integration (keep registry), rerun the full live-console mode with the unchanged assertions and capture observed ledger values before reporting READY.

The spec necessarily has a new hash because K3 requested ledger instrumentation; the confirmed c9e21169…cef19 remains the valid historical 992cff85 hash, not a new defect.
No push/deploy; existing integrator-owned output/ext-agents is untouched and unstaged. Commit and stop.
