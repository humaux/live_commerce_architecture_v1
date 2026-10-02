# ui-w0-shell — final closeout and K3 supplement

2026-10-02. **Implementation and supplemental evidence delivered; overall acceptance BLOCKED (14/15 commands pass).** Do not merge on this report alone. The one remaining failure is the unchanged real horizontal-scroll assertion in the legacy ledger test.

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ui-w0-shell`; branch `unit/ui-w0-shell`.
- Original unit base: `b4223c8448d4fabf5f6f5f7de19e84b0d99bad40`; closeout started from `9877a47f`.
- Final tested source: **`a6449fb33ce3050d6e385b3453130b7e469bf0d0`**. All 15 commands below ran sequentially on this fixed source. Only evidence/docs changed afterward.
- No push, merge, deployment, production host or credential access. Product Go/SQL unchanged. Only the explicitly authorized test Go fixtures were adapted.
- This replaces the earlier five-blocker ledger. Intermediate failures remain as provenance, not current results.

## Decisions 1–6

| Decision | Status | Evidence / boundary |
| --- | --- | --- |
| 1 Shell | IMPLEMENTED; final shell MOCK PASS | 220px dark rail; ten registered groups, only authorized groups with routes rendered; Settings pinned; real store/language/help/account controls. No global search, task centre or notification placeholders. Drawer below 1024px. Store initial + actual authenticated selected name, ellipsis and full-name title; no generic brand placeholder. |
| 2 Registry | IMPLEMENTED; node/browser PASS | 23 page paths mapped bidirectionally; navigation, titles, breadcrumbs and UI 403 generated from typed domain routes. Staff without catalog:read sees no product group/data, direct product URL renders 403. Unknown-store server 404 and mounted-shell explicit-invalid-ID 403 both asserted. Go remains authorization authority. |
| 3 Legacy bodies unchanged | IMPLEMENTED; regression acceptance BLOCKED | 46 committed pre-switch baselines (23 routes × 1586×992 / 390×844). No domain-body redesign. Restricted new selection styling to rail/topbar so legacy mint/ink selection and caret assertions remain exact. K3 reviewed earlier source/baselines; final visual approval belongs to integrator. Legacy scroll case still red below. |
| 4 packages/ui | IMPLEMENTED; architecture/browser PASS | Only shell, CSS Module and tokens; no speculative table/form library. Comp 01 governs shell. Selection scope cannot override legacy page bodies. |
| 5 packages/format | IMPLEMENTED; node PASS | Stop-bleed amount/Taipei implementations moved, not duplicated; old modules re-export. NT$ integer display and whole-TWD input. Existing out-of-scope offenders remain on shrinking allowlists. |
| 6 Copy | IMPLEMENTED; node/browser PASS | Typed zh-TW / zh-CN / en copy, key parity tests, three-language geometry and supplemental screenshots. |

## Closeout rulings and K3 supplement

| Item | Status / implementation |
| --- | --- |
| Legacy fixture identity + minimum permissions | IMPLEMENTED. Ledger's legacy backend does not expose session store-list; its UUID+UUID fixture token is not a signed identity token. Context-level MOCK transport intercepts only /api/stores. Payload is produced by actual isolated PG `identity.list_session_stores` under asserted `commerce_runtime`, exact fixture store checked. Real ledger data calls are not mocked. Only integration:read added for existing Settings reachability. Entry role:null + store:read/orders:read supports actual Dashboard landing. Registry locators replace flat-nav assumptions. |
| Ops fixture | PASS. Added only live:read for the existing Studio journey; all previous grants and assertions retained. Exact LINE locator avoids matching “OnLINE” in synthetic store name. |
| Customers denied | PASS. Central 403 plus no customers page/table/row/name; other finance/billing denial assertions retained. |
| Studio expired | PASS. Central sign-in recovery, no Studio data/action, localized recovery URL and actual link interaction retained. |
| Design dirty guard | PASS. Destination is Settings, already allowed by integration:read; no customers:read grant. Confirm/cancel/stay/leave assertions retained. After cancel, Escape closes the modal drawer before returning to editor input; drawer state asserted. |
| Store identity | PASS. Authenticated matching store only; explicit unknown ID never falls back to another shop. Initial mark and ellipsized name replace generic rail heading. |
| No expired / signed-out context | PASS. No brand, selector, breadcrumbs, scoped nav or domain body. Local logout, BroadcastChannel, storage and focus-triggered 401 clear state and abort reads. Address-bar store parameter removed. Component-lifetime expired latch prevents URL changes from relaunching a workspace read. Held old 200 response is awaited through handler settlement and two frames, then no-context/read-count asserted again. |
| Evidence hygiene | DONE. Root failure.png/failure.txt moved to red/pre-closeout-failure.*. Subsequent red captures also under red/, with provenance in red/README.md. No failure artifacts presented as accepted screenshots. |
| Missing K3 captures | DONE. Three-language 403, account menu including sign out, active child-route group expansion, and 390px measured touch-target annotations. Actual DOM pixel measurements saved beside PNGs; no fabricated target resizing. |

Logout failure remains an error: the UI stays fail-closed after the pre-logout signal and shows the existing failure message. This is not a claim that a failed POST revoked the server session; the existing full-page sign-in link is the recovery boundary.

## Commits

All listed code commits end with `Co-Authored-By: Codex <noreply@openai.com>`.

| SHA | Change |
| --- | --- |
| 3a2f3d2 / d6f4ad2 | Pre-switch baseline harness and public-page capture correction |
| b2dd0e3 | Single amount/time implementation moved to packages/format |
| 3f28c26 | Registry, shell, tokens and architecture/browser gates |
| 947474c / a4e0cec / b89bd75 | Earlier visual/locator corrections, public titles and invite metadata conflict fix |
| 9eca57b / 9826388 / 37b8eb9 / 2d2f3fd / 52486bd | Earlier route locator, token and explicit session-expiry fixes |
| b0c45381 | Authorized minimum-permission identity fixtures |
| 02f0ee0e | Central 403/recovery and permitted dirty-guard journeys |
| f9540b93 | Scope shell selection styling; exact LINE and drawer interaction corrections |
| c5dd5390 | Authenticated store brand, session-context invalidation and supplemental screenshot harness |
| 17f1b842 | Preserve full-visibility locator assertion; include 1024px scroll boundary and measurements |
| a6449fb3 | Expired URL cleanup with lifetime latch; settled delayed-read negative; server/client invalid-store checks |

The final evidence commit is the commit containing this SUMMARY: `git log -1 -- output/ui-w0-shell/SUMMARY.md`.

## Final commands and exit codes

Authoritative ledger: [verified-exits.tsv](verified-exits.tsv), timestamped and pinned to a6449fb3. **14 exit 0; 1 exit 1.** Parent orchestration exit is not substituted for individual gate exits.

| Exact command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | [log](verified-test-node.log) |
| `pnpm --filter admin exec tsc --noEmit` | 0 | [log](verified-admin-tsc.log) |
| `bash scripts/dev/check-gates.sh` | 0 | [log](verified-check-gates.log) |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | [log](verified-browser-admin-shell.log) |
| `bash scripts/dev/test-local.sh --browser-admin-legacy` | 1 | [log](verified-browser-admin-legacy.log) |
| `bash scripts/dev/test-local.sh --browser-ops-polish` | 0 | [log](verified-browser-ops-polish.log) |
| `bash scripts/dev/test-local.sh --browser-customers-billing` | 0 | [log](verified-browser-customers-billing.log) |
| `bash scripts/dev/test-local.sh --browser-studio-ui` | 0 | [log](verified-browser-studio-ui.log) |
| `bash scripts/dev/test-local.sh --browser-design` | 0 | [log](verified-browser-design.log) |
| `bash scripts/dev/test-local.sh --browser-catalog-core` | 0 | [log](verified-browser-catalog-core.log) |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | 0 | [log](verified-browser-catalog-media.log) |
| `bash scripts/dev/test-local.sh --browser-promotions` | 0 | [log](verified-browser-promotions.log) |
| `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` | 0 | [log](verified-browser-merchant-orders-ui.log) |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | [log](verified-browser-cvs.log) |
| `bash scripts/dev/test-local.sh --browser-meta-connect` | 0 | [log](verified-browser-meta-connect.log) |

| Gate | Final status |
| --- | --- |
| G-UI1 registry | PASS: node + check-gates; route/page bijection, copy, identity, permission, spec and no literal nav array checks. |
| G-UI2 geometry / roles | PASS: shell mode, 24 viewport/locale combinations (specified seven widths plus 375px), long store names, 13 destinations clicked per case, real drawer/navigation/store-switch interactions and negatives. |
| G-UI3 format/copy | PASS: compiler-AST gate, shrinking allowance, negative tests. |
| G-UI4 accessibility | PASS for shell chrome: axe no serious/critical findings, keyboard order, focus trap/return, ≥44px targets. Not certification of all legacy domain-body accessibility. |
| G-UI5 architecture | PASS: fetch boundaries, import/cycle/line limits and shrinking allowance checks. |
| G-UI7 legacy modes | **BLOCKED**: 10/11 pass, legacy ledger scrolling failure remains. No assertion removed or weakened. |
| Visual QA | Earlier K3 report PASS (P1=0/P2=0) at 52486bd; final supplemental captures ready. Refreshed K3 and human baseline/final approval NOT_RUN by this task, per integrator ownership. |

The specification has no separately numbered G-UI6. Approved comp 01 remains the visual reference; no new visual direction was introduced.

## Remaining blocker — legacy ledger actual overflow

Final `browser-admin-legacy`: ledger **18 passed / 1 failed**; identity and entry sub-suites PASS.
Exact failure: `tests/admin/visual-states.spec.ts:67`, `expect(overflowWidth).toBeGreaterThan(0)`.

The real fixture table fits at every probed responsive width:

| Viewport | scrollWidth | clientWidth |
| --- | --- | --- |
| 1100 | 830 | 830 |
| 1024 (added W0 rail boundary) | 754 | 754 |
| 900 | 866 | 866 |
| 820 | 786 | 786 |
| 740 | 706 | 706 |
| 681 | 647 | 647 |

Evidence: [red/ledger-scroll-widths.json](red/ledger-scroll-widths.json), identical to generated `output/playwright/ledger-review/scroll-widths.json`.
Detailed final failure: `output/playwright/admin-ledger-20261002T133732.017301000/playwright.log` and its local failure screenshot/trace.

Stop line: the current fixture/layout does not exercise overflow; this does not prove the wheel-scroll behavior. No injected CSS, fake overflow, removed width, weakened ratio/scroll/color assertion or domain-body change was used. Two targeted adaptations were exhausted; the final mandated regression re-recorded the same failure, not another attempted fix. **Integrator must adjudicate whether to authorize a genuine overflow-producing fixture or another scoped test/layout change.** Such a change is outside the current permission-metadata/locator-only fixture authorization. Do not declare G-UI7 green.

Intermediate merchant-order `Execution context was destroyed` failure is preserved in `closeout-browser-merchant-orders-ui.log`; both the 17f1b842 run and final a6449fb3 run pass without changing that test. This is preserved timing-risk provenance, not a claimed root-cause fix.

## Screenshot package

[screenshot-manifest.json](screenshot-manifest.json): **61 final MOCK PNGs**, exact dimensions + SHA-256, pinned to a6449fb3; hashes independently rechecked.

- 24 `shell-{locale}-{width}x{height}.png` + 12 `drawer-...` captures; 37th is `session-expired-en.png`.
- 6 `forbidden-{locale}-{1586,390}.png`.
- 6 `account-open-{locale}-{1586,390}.png` (including sign out).
- 6 `subroute-expanded-{locale}-{1586,390}.png` (Inventory child route active).
- 6 `touch-targets-{locale}-390-{topbar,drawer}.png` plus **6 JSON measurement files**.
- Three locales throughout: zh-TW, zh-CN, en. Supplement desktop 1586×992; phone 390×844.
- Pre-switch 46 baselines remain untouched under `tests/admin/baselines/w0/`.
- [red/README.md](red/README.md) explains pre-closeout, initial supplement and intermediate unknown-store red images. Intermediate final-*.log/final-exits.tsv are not the authoritative final-source ledger; use verified-*.

Root visually inspected expired, 403, account, active child route and touch annotation captures; independent reviewer also inspected representative captures. Narrow annotation labels can wrap on the 390px topbar; the JSON keeps exact readable measurements. This is test-overlay presentation, not a product control-size defect.

## Independent review, traceability and boundaries

- Root: Codex, runtime model not exposed, no model override; permitted writes only this worktree. Initial/closeout bases above.
- Independent read-only `w0_closeout_review`: originally configured gpt-6-sol/high; reviewer self-reported gpt-6.1-sol/inherited in final review, so actual runtime mismatch is not asserted away. Task `d08b14a6-91e1-4b3c-97ce-2316063c2cf2`, base17f1b842/head a6449fb3, write paths none. Final two-file diff: no new P0/P1. Earlier review suggestions about stale URL and weak delayed-read settling were implemented and root-retested.
- Read-only `r5_gate_paths` (gpt-6-luna/medium, no code writes) corrected its earlier permission note using supersedes; current Humaux memory `75beb6c9-5fb7-45ce-bcef-a46a4c479e5a`. Settings needs integration:read, entry Dashboard needs orders:read; never add unrelated customers permission.
- Humaux fix records: `acc50327-aa2f-48df-9c81-63f1d7fd2f85` for fixture/selection causes; `ad68f7de-de3a-4062-947d-aa7cc0b29804` for final expired-URL latch. 14 changed source files submitted to graph; index reports 11 parsed files/135 entities. Confirmed why-links to TestBrowserAdminLedgerFixtureChain and WorkspaceFrame. CSS/MJS-wide symbol coverage is not claimed.
- Impeccable audit-first isolated chrome changes from old page styles; Frontend Architect kept the existing React/Next/session-event/API boundary rather than adding a new auth/router layer. Supplemental detector output `supplement-detector.json` is empty. No K3 rerun was performed by this task.
- Go/PG browser modes use isolated fixtures and real local application/database paths where their logs state so; providers/IdP/Meta are MOCK. New shell gate itself is MOCK and its rendered UI 403 does not prove a server HTTP 403.
- NOT_RUN: production/LIVE operations, external provider sandbox/real credentials, refreshed K3, human baseline/final visual approval, full release gates beyond requested modes; optional R04 LiveKit binary branch unset; CVS provider SANDBOX explicitly skipped. CVS **WebKit ran and passed**.
- Cleanup postflight: no owned browser/Next/Go process with this worktree cwd remained; only the inspection shell/lsof/awk. Foreign `lc-focused-45274` verified as `home-cod-fix` via PID cwd and left untouched; pre-existing Humaux PG/Qdrant untouched. No foreign containers/directories were removed.
- Logs/evidence scanned for Stripe secret patterns, Meta EAA tokens, Bearer headers and session-cookie values: no matches. Local untracked integrator `REPORT-w0-vqa.md` and unrelated/generated `output/admin-ui-fixes/` were not added, edited or deleted by this evidence commit.
- Own failure artifacts and local Playwright diagnostics retained. No trace archives or private configuration bulk-added to Git.
