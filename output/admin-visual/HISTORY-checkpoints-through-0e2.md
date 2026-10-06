# admin-visual — verification in progress

Status: **IN_PROGRESS — final gate wave after approved harness integration**. No push or deployment.

## Authoritative current checkpoint

- Source: **0e2b5c55b7f01225c0f3fd3960ad37aa063c7fb1**, rebased onto approved `r3/integration` snapshot **3ed634be**. This includes the harness owner's visible-rectangle correction82eebefb; no UI-unit edits to `tests/ui/**`, Go or migrations relative to this base.
- FullNode, adminTS and strict check-gates (including headers) returned **0** on0e2. Final **33** browser modes are running serially with fixed source; actual exits accumulate in `gates-20261005T101241Z/results.tsv`. No source/HEAD changes during this wave.
- Before this rebase: product-editor0/live-claims0 at830b86f9; targeted existing-product sweep **95/95 clicks passed**,3viewport/locale units,0journeys. It is a **focused** pass despite the runner's generic all-routes closing sentence. Evidence in `sweep-focused-830b86f9/`.
- The earlier complete a4b sweep was **red**:123units/0load failures,977clicks=953pass+4fail+20skip,18/18journeys. Four image-readiness no-effect cases are retained in `sweep-red-a4b456b4/`. Their repair exposes the existing observed current section and real target; no fake mutation, changed command, or harness waiver. A test-selector defect was independently caught and corrected before the830b green.
- Old clipping-blocker conclusions below are historical. The approved harness fix is now integrated, but **new full visual/full sweep acceptance is not claimed before actual completion**.
- Remaining delivery work: final gate results, refreshed35-finding exact-source evidence and independent review/handoff. The next live-console unit remains NOT_STARTED.

Everything below records earlier checkpoints and preserved red history, not the authoritative current acceptance state.

## Current integration checkpoint (2026-10-05)

Requested rebase is complete onto `65902dbe` (local `r3/integration`, containing `fd6ce9ab`). Current source is `8ee1127c0531f672ca843a512f00355909f55519`. See `REBASE.md`:36commits replayed, two header/import conflicts resolved,208 generated outputs restored byte-for-byte. All earlier SHA-specific results below are **historical**, not evidence for the changed baseline.

Integrator P2 fixes implemented: zh-CN legal copy consistently uses“评论收单”; the nonblocking“至少3张图片”recommendation is restored with distinct three-locale text, while the required≥1-image rule is unchanged. New assertion REDexit1→GREENexit0 (8tests), logs `integrator-p2-red-8fcb3745.log` and `integrator-p2-green.log`. Optional advice had been removed during duplicate-checklist cleanup; its separate usefulness is now retained without duplicate labels. Legal text remains **需 owner/律師審閱**.

New frozen visual rules (including R9) are being measured on `8ee1127c`; no source changes while this run executes. Root's extra file headers are committed; two isolated comment-only cards cover the other89 files. Full new-source gates, final35-finding evidence and handoff remain pending. The next live-console unit is explicitly NOT_STARTED.

Latest pinned runtime source is now **a4b456b42a9df44fa054db8d18937cf73c6bca61**. The89 header-only files are integrated (`15f30f70`, `d40204fb`); root independently proved every added line is a comment and every preexisting byte is preserved. `LC_HEADERS_STRICT=1 check-headers`, fullNode, adminTS and check-gates all returned0. The former line about8ee1127c running is historical.

8ee1127c visual exit1 captured294images with51blocking instances:46 actual clipped locale labels,3 raw-tail R6 and2 raw-tail R9. Mobile locale width is corrected in `4ca41c1a`, with runtime RED1→admin-shell GREEN0 across eight widths and three locales. R9's actual visible margin is16px, independently measured by the browser on390zh-CN/zh-TW; the reported7px uses an off-scrollport raw tail. See `R6-BLOCKER.md`; frozen failures remain unwaived.

Current batch `gates-20261005T085441Z`: admin-shell0, catalog-core0, platform-site0; product-editor1 (13passed, obsolete image-substring count contradicts restored3-image recommendation) and live-claims1 (new hint test omitted clearing an already selected product). Required changes are **test-only**: assert exactly one required image item and one distinct3-image recommendation; use a real clear-selection before disabled-SKU assertions. They will be applied only after the current pinned visual/full sweep finishes. No business predicate or threshold is changed. Current final full scan/sweep still pending.

## Scope and source

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/admin-visual`, branch `unit/admin-visual`.
- Base: product-editor-visual `897e0fd6`; frozen visual harness import `70d9d1d2`.
- Owner approved `packages/ui/src` presentation-only expansion, in addition to `apps/admin/**` and `tests/admin/**`. No business/auth/API/Go/SQL changes are authorized by this unit.
- Previous pinned regression source: `e85654d00f684a855f7600d5af94a3ac29c67f46`. The only change from `bb7e062e` was the CVS screenshot option `scale: "css"`; application source was identical. These are historical after the integration rebase.
- Reviewed follow-ups are now integrated: editor end spacing `5c1d695e`, real print response hold `f4fb280b`, inspector clearance `37091027`, MOU05 click `45ee7522`, status compatibility `21fd4472`, mobile targets/columns `b4fb12dd`, empty attribution wording `8631bcc4`, canonical ledger Badge `bb7e062e`.

## Evidence already obtained

- Fresh BEFORE corpus: `output/ui-visual-audit/20261005T052825Z`, source `70d9d1d2`; retained admin screenshots under `before/`. The externally cited original `045559Z` directory is currently absent; its references are preserved, not substituted silently.
- First integrated RED: `20261005T055641Z`, source `ae8fcc25`; blocking findings reduced from 50 to 11, R4 from 181 to 0.
- Candidate GREEN: `20261005T063819Z`, source `5a895dea2c`; unchanged `--browser-visual-lint`, exit **0**, 294/294 screenshots. R1/R2/R3/R4/R6/R8 all **0**; non-blocking R5=368, R7=66. This is not business acceptance and the CVS external-placeholder route is not genuine print-page coverage.
- Independent reviewer directly inspected 54 candidate after-shots in the shared-shell/form/navigation scope across three locales and both widths: no P1/P2 found in that scope, not a review of every route or interaction.
- Product editor candidate: 12 actual-click tests, mandatory frozen CC12 and REAL_PG readback passed at `5a895dea`; subsequent stronger repeated-feedback test exposed an additional boundary.
- `--browser-live-claims`: exit **0**, 20 cases across three locales at `5a895dea`.

## Current final-gate attempt

Every browser command is `LC_TEST_LOCK_WAIT=14400 LC_SWEEP_WORKERS=1 PRODUCT_VISUAL_PHASE=after bash scripts/dev/test-local.sh <mode>`.
Actual per-command exit codes, timings and source SHA are appended to each `gates-*/results.tsv`; each row links its full log. Missing rows mean **NOT_RUN/pending**, never PASS. `064922Z` has 31 completed modes at `1ded5877`; its later sweep was deliberately interrupted after reviewed fixes were ready, and that source's final visual scan was NOT_RUN (see `064922Z/INTERRUPTED.md`). `073028Z` is the corrective rerun at `bb7e062e`.

Static commands at `1ded5877`:

| Command | Exit | Log |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | `logs/test-node-1ded5877.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `logs/tsc-1ded5877.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `logs/check-gates-1ded5877.log` |

Static commands repeated at `bb7e062e`: test-node **0** (403 passed, 0 failed, 7 Node report blocks); admin tsc **0**; check-gates **0**. Logs have the corresponding `-bb7e062e.log` suffix.

Corrective runtime results read back at `bb7e062e`: product editor **0** (14 browser cases plus frozen CC12 and REAL_PG); admin legacy **0**; merchant-orders UI **0** (10/10); catalog media **0** (20 cases, four viewport/locale cells, exact DB readback). Its CVS Chromium passed; WebKit screenshots were RED because default device pixels were 2x CSS width (3172 vs 1586). Test-only `e85654d0` uses CSS pixels without changing dimension/overflow assertions: complete `--browser-cvs` is now **0**, both Chromium and WebKit.

Static commands repeated at `e85654d0`: test-node **0** (403 passed, 0 failed), admin tsc **0**, check-gates **0**, `git diff --check` **0**. See `logs/*-e85654d0.log`. Final serial results are in `gates-20261005T074219Z/results.tsv`.

The remaining prior-source modes are also being rerun on `e85654d0`, tracked separately in `gates-20261005T080801Z/results.tsv`. Product-editor and admin-legacy already returned **0**. This second runner initially overlapped the full sweep despite individual script locking; its parent was paused while the current merchant-orders test finishes, so no further own builds start during the full sweep. No other process or lock was touched. Any timing/environment failure remains in its real log and is not silently ignored.

### Final frozen visual result: BLOCKED, not waived

`--browser-visual-lint` at `e85654d0`: **exit 1**, complete **294/294** captures in `output/ui-visual-audit/20261005T074421Z/`. Admin R1/R2/R3/R4/R5/R8 are **0**; R6 has **3** instances, all on `products-product`, English, 1586×992. R7 has 66 intentional long-store-name ellipsis warnings. Global R5=328 belongs to storefront, outside this unit.

The three R6 instances concern raw rectangles of partially clipped axis controls (top 897.5, height 44) against footer buttons (top 936), producing a 5.5px raw overlap. The screenshot clips editor fields at approximately y907 and starts the footer at y924. The frozen collector computes clipping in `vis()` but stores raw target rectangles for R6. This discrepancy is undergoing independent reproduction. **The failed gate remains failed.** No detector edit, arbitrary spacing, sticky classification, hidden control, or threshold relaxation is used to manufacture acceptance. The initial commentary incorrectly attributed these instances to inventory; the actual finding is the English product editor.

### Actual CVS print evidence

- Chromium: `cvs-print/2026-10-05T07-43-12-872Z/` (12 images).
- WebKit: `cvs-print/2026-10-05T07-43-55-025Z/` (12 images).
- Three locales × 1586/390 × merchant-created and opening-before-response states. PNG hashes and CSS widths were independently checked against both manifests.
- These hold the real BFF response before releasing it unchanged; signed provider POST assertions remain. They do not claim provider-layout or physical printing coverage. The six frozen visual-runner CVS placeholder images are excluded from actual after-shot evidence.

### Preserved earlier red evidence / corrective readback

1. `338bdeb9`: UNKNOWN retry existed outside the pane viewport. Fixed and actual GREEN at `5a895dea`; no receipt/writer changes.
2. `f2b752f7`: second identical image-required validation stayed off-screen (ratio 0), `product-feedback.acceptance.ts:72`. `041ca2a7` fixes the repeated-attempt trigger. At 390px a later run exposed fractional end clipping (ratio 0.996358). `5c1d695e` adds real scroll-end spacing; the strict ratio **1** assertion passes at both widths in the `bb7e062e` product-editor run.
3. Old CC12 count-only wait could accept the previous 25-row page before the next cursor rendered. `1ded5877` requires the first row ID to change, retaining all **25 + 25 + 10** assertions.
4. The first ADM35 capture held outbound navigation, so Playwright waited for navigation and could not read the old DOM. `f4fb280b` holds the real BFF response instead and accurately captures the **opening** state, not provider layout. Both engines pass at `e85654d0`.
5. Legacy inventory tray: the enlarged 44px close button overlapped the heading by 5px. `37091027` reserves real close-button clearance. Canonical scrollbar colors replace the old color expectation; real overflow, wheel movement and thin-width assertions stay intact. Legacy passes at `bb7e062e`.
6. Catalog media: the archived SKU was present and visibly labelled archived, but the shared Badge omitted old `.status.archived`/`.status.active` classes used by the frozen gate. `21fd4472` preserves those presentation classes; `bb7e062e` removes the remaining duplicate status implementation. Catalog-media passes, without data or assertion changes.
7. MOU05: a remaining unconditional More Filters click closed an already-open disclosure. The existing conditional real-click helper replaces only that line; all keyboard, geometry, privacy, paging and target-size assertions are unchanged.
8. WebKit's buyer/order/payment/merchant-buyer/password-auth steps passed in the earlier red run; its remaining CVS screenshot issue is corrected as above. The complete WebKit umbrella is being rerun at `e85654d0`. No provider assertion is removed.

### Independent review qualifications

- Final-source independent reviewer directly inspected **48 current images**: inventory and attribution (three locales, both sizes), five affected mobile target shapes, and all 24 actual CVS states. No confirmed P1/P2 in that scope. Report: `reviews/final-shared-e85654d0.md`. This is not all-35 or all-294 acceptance; it does not waive R6 or claim real horizontal dragging/native widget/physical print verification.
- An independent source/screenshot check confirmed the raw-rectangle/clipped-region R6 discrepancy; arithmetic probe is MODEL_ONLY, browser hit-testing NOT_RUN. See `R6-BLOCKER.md`. A ruling has been requested; failed visual exit remains **1**.
- Candidate admin R5 had 40 instances across five genuine shapes (CAPI label, customer-order links, dashboard-order links, dashboard all-orders link, domain-origin link). `b4fb12dd` repairs actual target sizes; final admin R5 is **0**. R7 retains 66 intentional long-store-name ellipsis warnings.
- ADM22: resting date text follows the page locale; focused native calendar/entry UI remains browser/OS controlled. A locale-independent custom calendar was not introduced. Native focused-picker locale validation is **NOT_RUN**, not silently passed.
- ADM31: the duplicate advertising budget/report safety sentence is required by frozen `ads-ui.md` section 12 and `ads-model.test.ts:80`; it is intentionally retained. `8631bcc4` deduplicates empty attribution selection wording without changing values/handlers or the disabled reason.
- ADM27 has collapsed advanced filters and compact IDs/badges. The export explanation remains visible; removal or hiding of its data/privacy limits is not claimed.
- ADM29: original mobile column hiding remained in early candidate screenshots; `b4fb12dd` exposes all columns through the existing horizontal scroll surface instead.

## NOT_RUN / acceptance boundaries

- Same-source final functional reruns are in progress;35-finding before/after observations are assembled but independent retirement/acceptance remains pending, including the frozen R6 ruling.
- Full interactive click sweep is distinct from visual-only capture and is pending in the current run.
- CVS provider layout, physical printing, ECPay/Stripe/Meta SANDBOX and LIVE, production deployment, actual mailboxes: **NOT_RUN**. All invoked browser fixtures are isolated local services with MOCK external providers.
- The default Node suite reports the optional live media runner NOT_RUN when `COMMERCE_R04_LIVEKIT_BINARY` is unset.
- No thresholds were relaxed, failed assertions removed, or fixture DOM/CSS injected to manufacture a visual pass.

## Traceability

The35-finding before/after map is now in `evidence-map/observed-after-e856-20261005T074421Z/FINAL-EVIDENCE.md` with complete metadata in `evidence-pairs.json`. It contains156 real after-images,162 route/locale/size pair records (six excluded CVS placeholders),24 references to real CVS captures, and two red crops. Root independently verified156 copied image hashes; the failed verdict is unchanged. Every finding is **OBSERVED_AFTER**, not independently retired. See `evidence-map/ROOT-COPY.md` for path relocation provenance.

76 changed TypeScript/test files were submitted to Humaux code indexing in bounded batches, followed by six changed files and the final e856 CVS test delta; layout/editor/list decisions were linked to indexed entities. Root task/canvas and incremental fix memories are maintained under `codex-admin-visual`. Final command table and full interactive sweep remain in progress.
