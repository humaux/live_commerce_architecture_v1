<!-- Purpose: W3-U3 scoped implementation checkpoint, evidence and the unresolved Node-host gate.
Depends on: frozen W3-U3 brief, live-console-v1 §7.4 and the current-source logs below.
Used by: integrator scope ruling, independent review and CI dispatch; not a merge approval. -->
# W3-U3 comment label print — CHECKPOINT / BLOCKED

- Branch: `unit/w3-u3-comment-label-print`; base `d5c865a5` (origin LC-U2a; fetched trunk already ancestor).
- Source: `5b01bf58` (feature `05079102`, capture fix `38375e72`, per-ref assertion `5b01bf58`). Author: Codex-1; exact host model identifier unavailable; no delegates.
- Scope: the four brief files plus **only** `playwright.config.ts` and `tests/foundation/browser_live_console_test.go`, explicitly approved by integrator. No product Go/SQL/OpenAPI or dependency changes. Frozen install exit 0.

## Implemented

- `CommentLabelPrint.tsx`: current in-memory keyword rows only; checkbox/single-label entries, native dialog preview, 60×40mm or A4 three-column print CSS. Native W0 controls/colors, three locales. The only browser storage write is a validated paper-size preference. No name/text/label payload upload.
- Reuses actual `inboxWrite`, session/CSRF boundary and A3 `{}` request; one distinct key per selected ref. Lost ACK retains the key in memory. Only confirmed counts produce badges; failed records do not block physical print. Text explicitly distinguishes a recorded action from physical printer success.
- `CommentStream.tsx`: seven entry-point lines; keyed store/session/privacy/reset boundary unmounts private label state. Rows no longer in the memory buffer cannot be printed. No second reader or reply writer.
- Go **test only**: isolated `labels` subtest uses signed ingress and the real intake worker to produce three ACCEPTED claims. Existing workspace suite still has 60 rows/one claim and unchanged assertions. A3 facts are cleaned before the fixture session (scoped tenant/store/session DELETE in test teardown only).
- Real browser gate checks all three names/keywords/quantities/times/short codes; each selected ref receives exactly one `{}` POST with a unique key; print-only DOM excludes console/heading/controls; counts survive a real A2 reload; storage has no comment text/name/ref; failed record still prints without a new badge; epoch reset removes preview.

## Evidence and gates

| Command | Exit | Result |
|---|---:|---|
| Synthetic-env `pnpm run build:admin` | 0 | `green-build.log`; same runtime files as current source |
| `LC_FOCUSED_TAGS=browser LC_BROWSER_LIVE_CONSOLE_ACCEPTANCE=1 LC_BROWSER_CONSOLE_GREP='W3U3 three label real A3 print zh-TW-390' bash scripts/dev/test-focused.sh '^TestBrowserLiveConsoleRealChain$/^labels$'` before product implementation | 1 | `red.log`; three real claims loaded, missing checkbox timed out at spec:48; baseline product `d5c865a5` |
| Same focused labels regex without grep | 0 | `labels-current.log`; **7/7 browser cases**, real Next/Go/PG + MOCK Graph, 1586/390 × zh-TW/zh-CN/en plus failure/reset case |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log`, 82 documented modes; source-header/architecture checks |
| `node /Users/luolimo/.codex/skills/impeccable/scripts/detect.mjs --json apps/admin/components/CommentLabelPrint.tsx apps/admin/app/print.css` | 0 | `design-static.json` = `[]`; static only, not independent visual approval |
| `bash scripts/dev/test-node.sh` | **1** | `node.log`: existing `inbox-review-host.test.ts` substitutes a React runtime without createContext/useContext/createPortal; importing the new component fails before existing hook tests run |

The first green browser run had 7 passing browser cases but **exit 1** during fixture FK cleanup (`labels-green.log`). The approved test teardown fix made the unchanged seven cases and teardown green (`labels-cleanup-green.log`). Original failed runs remain evidence, not PASS.

## BLOCKED: Node host scope

An async scope request asks to extend **only** `tests/admin/inbox-review-host.test.ts` and its isolation tests to support the standard React presentation APIs used by the label component. Current approval named the Playwright registration and Go fixture, not this shared Node host. It has not been modified. Do not stub away the product component, weaken hook assertions, or call the unit complete while test-node is red.

## CI gates / NOT_RUN

- Full `bash scripts/dev/test-local.sh --browser-live-console` (both unchanged workspace and new labels lanes).
- `bash scripts/dev/test-local.sh --browser-click-sweep`.
- `bash scripts/dev/test-local.sh --browser-visual-lint`.
- PR-selected modes from `node scripts/dev/pr-modes.mjs origin/r3/integration`, required CI and non-author review.
- Native print **DOM/media** tested; actual OS print dialog, pagination through a physical printer, printer drivers and LIVE Meta are **NOT_RUN**. `window.print` is intercepted only to count invocation; all HTTP writes/A2 reloads are real fixture paths.
- No push, deploy, provider mutation, production credentials or real buyer data. No tests run in parallel against PG. Own harness processes ended; shared locks/other agents untouched.

PR #16 round 1 was already delivered separately as `f313e418`; do not repeat it. `settings-followups` remains next after this unit's gate closure.
