# ui-w0-shell — final closeout and K3 supplement

2026-10-02. **The final authorized test-only blocker is resolved.** Five requested commands were rerun and passed; the other ten G-UI7 modes retain their prior passing evidence because this one-spec change cannot select or affect them. Integration, K3/human visual approval and release authority remain with the integrator.

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ui-w0-shell`; branch `unit/ui-w0-shell`.
- Original unit base: `b4223c8448d4fabf5f6f5f7de19e84b0d99bad40`; closeout started from `9877a47f`.
- Latest tested source: **`8abaaaa3abcb79712438a3cd2902b60d97c95e73`**, starting from `dedb43c8`. Only `tests/admin/visual-states.spec.ts` changed in this final ruling; no product files, CSS or Go/SQL changed. Five commands were rerun at this source. The prior full 15-command record remains pinned to `a6449fb33ce3050d6e385b3453130b7e469bf0d0`; its ten unaffected passing modes are explicitly carried forward, not reported as rerun.
- No push, merge, deployment, production host or credential access. Product Go/SQL unchanged. Only the explicitly authorized test Go fixtures were adapted.
- This resolves the earlier remaining scroll blocker under the owner's option B. Intermediate failures remain as provenance, not current results.

## Decisions 1–6

| Decision | Status | Evidence / boundary |
| --- | --- | --- |
| 1 Shell | IMPLEMENTED; final shell MOCK PASS | 220px dark rail; ten registered groups, only authorized groups with routes rendered; Settings pinned; real store/language/help/account controls. No global search, task centre or notification placeholders. Drawer below 1024px. Store initial + actual authenticated selected name, ellipsis and full-name title; no generic brand placeholder. |
| 2 Registry | IMPLEMENTED; node/browser PASS | 23 page paths mapped bidirectionally; navigation, titles, breadcrumbs and UI 403 generated from typed domain routes. Staff without catalog:read sees no product group/data, direct product URL renders 403. Unknown-store server 404 and mounted-shell explicit-invalid-ID 403 both asserted. Go remains authorization authority. |
| 3 Legacy bodies unchanged | IMPLEMENTED; affected regression PASS | 46 committed pre-switch baselines (23 routes × 1586×992 / 390×844). No domain-body redesign. Restricted new selection styling to rail/topbar so legacy mint/ink selection and caret assertions remain exact. Latest test adds one realistic SKU through the existing API into the isolated fixture; the unchanged scroll assertion now passes. Final visual approval belongs to integrator. |
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
| 8af8906d | Owner option B: one real long-SKU product in the visual test's disposable fixture; old assertions/widths unchanged |
| 8abaaaa3 | Align new setup assertions with catalog commands' actual 200 contract; verify returned product/code |

The final evidence commit is the commit containing this SUMMARY: `git log -1 -- output/ui-w0-shell/SUMMARY.md`.

## Final commands and exit codes

Latest five-command ledger: [overflow-final-exits.tsv](overflow-final-exits.tsv), pinned to **8abaaaa3**, **all exit 0**. Prior [verified-exits.tsv](verified-exits.tsv) records the earlier a6449fb3 run (14 pass / 1 fail). Parent orchestration exit is never substituted for individual gate exits.

In the following combined coverage table, the first five rows are fresh runs. The remaining ten rows are the unchanged modes' prior a6449fb3 runs: `playwright.config.ts` lists `visual-states.spec.ts` only in suite `ledger`, and only `--browser-admin-legacy` selects that suite. No shared fixture, configuration, product source or other spec changed. This is not a claim that all fifteen commands were rerun at one new SHA.

| Exact command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | [fresh log](overflow-final-test-node.log): 284 pass; optional R04 NOT_RUN |
| `pnpm --filter admin exec tsc --noEmit` | 0 | [fresh log](overflow-final-admin-tsc.log) |
| `bash scripts/dev/check-gates.sh` | 0 | [fresh log](overflow-final-check-gates.log): 56 modes covered |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | [fresh log](overflow-final-browser-admin-shell.log): 24 matrix cases, roles, store switch, axe |
| `bash scripts/dev/test-local.sh --browser-admin-legacy` | 0 | [fresh log](overflow-final-browser-admin-legacy.log): ledger 19 pass; identity and entry PASS |
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
| G-UI7 legacy modes | PASS for affected-change coverage: legacy freshly passes, other ten unaffected modes carry prior PASS. No assertion removed or weakened. |
| Visual QA | Earlier K3 report PASS (P1=0/P2=0) at 52486bd; final supplemental captures ready. Refreshed K3 and human baseline/final approval NOT_RUN by this task, per integrator ownership. |

The specification has no separately numbered G-UI6. Approved comp 01 remains the visual reference; no new visual direction was introduced.

## Resolved blocker — legacy ledger actual overflow

Prior `browser-admin-legacy`: ledger **18 passed / 1 failed** at `expect(overflowWidth).toBeGreaterThan(0)`. That prerequisite and all wheel, selection, caret and scrollbar thresholds remain unchanged. Latest run: **ledger 19 passed**, identity and entry PASS.

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

The owner subsequently authorized either a genuine narrower viewport or realistic long test data. Option A was measured first: 768/390/375/360 also had `scrollWidth === clientWidth` (734/356/341/326). See [red/phone-scroll-widths.json](red/phone-scroll-widths.json) and [overflow-probe-legacy.log](overflow-probe-legacy.log), exit 1.

Option B creates one synthetic merchant-style product and its **51-character** variant code `HA-RECHARGEABLE-BTE-BLUETOOTH-CHARGER-BLACK-TW-2026` via the existing BFF → Go → isolated PG commands, then navigates to the real SSR inventory projection. It checks both HTTP 200 responses, returned product/code and the rendered SKU. No read-response replacement, DOM mutation or injected styles. Existing widths and assertions are identical to the pre-ruling test. The first setup run mistakenly expected 201; corrected after reading `bodyRoute`'s 200 contract, and retained [red/long-sku-status-contract.log](red/long-sku-status-contract.log). This correction changes only a new setup assertion, not an existing gate threshold.

Final measured result: **1024px viewport; scrollWidth 795px; clientWidth 754px; actual wheel-induced scrollLeft 41px**. Scrollbar colors and `thin` width, original selection/caret RGB values, and hidden dev chrome assertions all pass. Evidence: [overflow-active-style-evidence.json](overflow-active-style-evidence.json), [overflow-scroll-widths.json](overflow-scroll-widths.json), [overflow-scrollbar-active.png](overflow-scrollbar-active.png), [overflow-ledger-passing.log](overflow-ledger-passing.log). Original run: `output/playwright/admin-ledger-20261002T152430.342698000`.

Cleanup ownership: the Go test stops its owned fixture/Next processes; the `test-local.sh` EXIT trap removes its exact labeled disposable PostgreSQL container, including the new SKU. No real merchant inventory is touched.

**Visible adjacent P2, not fixed in this test-only task:** in the long-SKU evidence the legacy SKU text overlaps neighboring numeric columns at 1024px. The authorized natural-scroll behavior is proven, but this is not whole-table visual approval. Existing `.sku-col` sizing/nowrap behavior is a product-layout follow-up for the integrator; no CSS change was made here.

Intermediate merchant-order `Execution context was destroyed` failure is preserved in `closeout-browser-merchant-orders-ui.log`; both the 17f1b842 run and final a6449fb3 run pass without changing that test. This is preserved timing-risk provenance, not a claimed root-cause fix.

## Screenshot package

[screenshot-manifest.json](screenshot-manifest.json): **61 refreshed MOCK PNGs**, exact dimensions + SHA-256, rerun on 8abaaaa3; hashes rechecked. The separate real-fixture overflow screenshot above is not part of this MOCK matrix.

- 24 `shell-{locale}-{width}x{height}.png` + 12 `drawer-...` captures; 37th is `session-expired-en.png`.
- 6 `forbidden-{locale}-{1586,390}.png`.
- 6 `account-open-{locale}-{1586,390}.png` (including sign out).
- 6 `subroute-expanded-{locale}-{1586,390}.png` (Inventory child route active).
- 6 `touch-targets-{locale}-390-{topbar,drawer}.png` plus **6 JSON measurement files**.
- Three locales throughout: zh-TW, zh-CN, en. Supplement desktop 1586×992; phone 390×844.
- Pre-switch 46 baselines remain untouched under `tests/admin/baselines/w0/`.
- [red/README.md](red/README.md) explains pre-closeout, initial supplement and intermediate unknown-store red images. Earlier final-*.log/final-exits.tsv and verified-* remain historical; use overflow-final-* for this last five-command rerun.

Root visually inspected expired, 403, account, active child route and touch annotation captures; independent reviewer also inspected representative captures. Narrow annotation labels can wrap on the 390px topbar; the JSON keeps exact readable measurements. This is test-overlay presentation, not a product control-size defect.

## Independent review, traceability and boundaries

- Root: Codex, runtime model not exposed, no model override; permitted writes only this worktree. Initial/closeout bases above.
- Final test-only task `e1265939-92cb-4ec6-8eed-71075d1e7d8b`, base dedb43c8/head 8abaaaa3, writer paths only `tests/admin/visual-states.spec.ts` and `output/ui-w0-shell/`. Independent readonly explorer `w0_scroll_review` configured gpt-6-luna/low; actual runtime not independently exposed. Scope review memories `2a479f5e-145c-40d9-8802-2ed4afa2664a` and `991b834b-2385-42ff-be17-92ad6074e18a`: unchanged thresholds, only legacy affected, schema-valid data and actual response contract. Reviewer did not run PG/browser; root ran all five required commands. The cleanup wording nit is clarified above.
- Independent read-only `w0_closeout_review`: originally configured gpt-6-sol/high; reviewer self-reported gpt-6.1-sol/inherited in final review, so actual runtime mismatch is not asserted away. Task `d08b14a6-91e1-4b3c-97ce-2316063c2cf2`, base17f1b842/head a6449fb3, write paths none. Final two-file diff: no new P0/P1. Earlier review suggestions about stale URL and weak delayed-read settling were implemented and root-retested.
- Read-only `r5_gate_paths` (gpt-6-luna/medium, no code writes) corrected its earlier permission note using supersedes; current Humaux memory `75beb6c9-5fb7-45ce-bcef-a46a4c479e5a`. Settings needs integration:read, entry Dashboard needs orders:read; never add unrelated customers permission.
- Humaux fix records: `acc50327-aa2f-48df-9c81-63f1d7fd2f85` for fixture/selection causes; `ad68f7de-de3a-4062-947d-aa7cc0b29804` for final expired-URL latch. 14 changed source files submitted to graph; index reports 11 parsed files/135 entities. Confirmed why-links to TestBrowserAdminLedgerFixtureChain and WorkspaceFrame. CSS/MJS-wide symbol coverage is not claimed.
- Impeccable audit-first isolated chrome changes from old page styles; Frontend Architect kept the existing React/Next/session-event/API boundary rather than adding a new auth/router layer. Supplemental detector output `supplement-detector.json` is empty. No K3 rerun was performed by this task.
- Go/PG browser modes use isolated fixtures and real local application/database paths where their logs state so; providers/IdP/Meta are MOCK. New shell gate itself is MOCK and its rendered UI 403 does not prove a server HTTP 403.
- NOT_RUN this final test-only round: the ten unaffected G-UI7 modes (prior PASS retained, not rerun), production/LIVE operations, external provider sandbox/real credentials, refreshed K3, human baseline/final visual approval, full release gates beyond requested modes; optional R04 LiveKit binary branch unset. Earlier CVS provider SANDBOX explicitly skipped; earlier CVS **WebKit ran and passed**, not rerun now.
- Cleanup postflight: no owned browser/Next/Go process with this worktree cwd remained; only the inspection shell/lsof/awk. Foreign `lc-focused-45274` verified as `home-cod-fix` via PID cwd and left untouched; pre-existing Humaux PG/Qdrant untouched. No foreign containers/directories were removed.
- Logs/evidence scanned for Stripe secret patterns, Meta EAA tokens, Bearer headers and session-cookie values: no matches. Local untracked integrator `REPORT-w0-vqa.md` and unrelated/generated `output/admin-ui-fixes/` were not added, edited or deleted by this evidence commit.
- Own failure artifacts and local Playwright diagnostics retained. No trace archives or private configuration bulk-added to Git.
- Final-round postflight: no owned shell runner, admin-fixture or Next process remained. The five observed Next processes had `cvs-ui/apps/storefront` cwd, not this worktree, and were left untouched. No `lc-foundation-test-*` container remained. Machine PG/browser lock released. Copied overflow logs only normalize terminal trailing whitespace; no test result text was removed.
