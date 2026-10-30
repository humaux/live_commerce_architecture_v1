# PARITY-STATES-FIX1 — PR #356

Date: 2026-10-31 (Asia/Shanghai). Status: implementation prepared; PR CI PENDING. Author assessment E0; permitted static checks E1. No completion or E4 claim.

## FIX1 relay 1 — current-head Foundation blocker

Re-fetched and merged `origin/r3/integration` before editing: already up to date at `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425`. Inspected PR head `cd6935deeb91b171d4777dcdf37c832500a6b2a1`; Foundation run **41106548606**, job **124237773182**, tested synthetic merge **64811bc3648a3676d933451847e2a9c8d3069088**. Its Node suite reports 336 tests, 335 passed, one failed. The earlier browser rows below describe the previous head, not current acceptance. Gates **41106548811** was still queued when read; no new-head browser outcome is claimed.

|Failing job|Failing test / assertion|Original evidence|Class and directed change|
|---|---|---|---|
|124237773182 — Foundation, run 41106548606|`tests/ci/gate-waits.test.mjs:18`: browser gate scripts await every started wait before the next await; `shop-real-gate.mjs:451 cartRequested is awaited only after another await`|Original failed job log: 2026-10-30T18:29:31.6472983Z–18:29:31.6480774Z; exact assertion excerpt appended to `FIX1-ci.log` and `red.log`|**(b) regression in this PR's browser helper.** Sequential click/wait leaves the request wait unobserved if the click fails; declaring it outside `try` also makes the lexical gate see the later alert assertion as its next await. Register the same request inside `try`, before the click, then await both in one `Promise.all`. Preserve the held read, its finally release, every alert/recovery/reload/no-write assertion, fixtures and deadlines.|

This is directed attempt **1 for this wait-order blocker**, following its first observed CI assertion RED; the prior cart timing and parcel-copy fixes are separate blockers whose current browser result remains PENDING. Acceptance is the unchanged wait gate plus this PR's own Foundation/Gates and browser receipts at the next head. If the same wait assertion survives two directed fixes, STOP/BLOCKED; if either original FIX1 browser assertion remains red, follow the card's STOP rule.

Local static commands: `node --check tests/storefront/shop-real-gate.mjs` exit 0; browser ownership regeneration and `--check` exit 0 (54 modes), E1 only. No imports, product source, locale strings, gate code, thresholds or fixture payloads changed in this relay. The generator is expressly required by the card; no local tests or servers ran. Only this existing browser spec, generated ownership and this card's evidence documents are touched. No newly out-of-scope file; the prior parcel-spec/header list below still requires integrator review. Humaux unavailable: no store/canvas/coord claim. Rollback is a revert of this relay commit; integrator owns CI-completion wakeup.

## Authority and checkout

- Authority: integrator's FIX1 instruction in this session; current repository code/contracts; original GitHub run/job logs and downloaded browser evidence. Signed denial copy is implemented in `packages/i18n` and `AdminForbidden.tsx`, with frame mapping `1429:34986` / `1429:35006` in the existing DELIVERY. No redesign or Figma writes.
- Started at `88c147747271e71b60078afb647e03d1e56c2b83`, branch `unit/parity-states`, this worktree only. Merged `origin/r3/integration` at `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425` first, merge commit `107418797`. Resolved only WorkspaceFrame header wording and regenerated browser ownership. Upstream changes retain their upstream ownership.
- Read AGENTS.md, 架构.md §§0/1/2/26/27, invariants I01/I02/I05/I06/I18/I21, storefront-v2 E5/G3 and Stripe buyer UI uncertainty rules. Backend scope/payment authority and all no-replay controls remain unchanged.
- Base Gates `41105181453` was **queued**, with no jobs/conclusion, when inspected. No identical-base failure or base-green claim is made.
- Role: dispatched UI implementation worker; Codex GPT-6, inherited effort, no override or subagents. Humaux tools are absent from the callable inventory: search/rejected/coord/store/canvas/code-index are UNKNOWN, not claimed as performed. File receipts are the available handoff; integrator owns the CI-completion relay.

## Complete failure classification

Inspected RED run **41079438742**, head `88c147747271e71b60078afb647e03d1e56c2b83`, tested synthetic merge `b91e125e64bc87196879b25ad5f4a0e24e4a147b`. Raw failed-job logs and artifacts are local under `fix1/`; compact original excerpts are committed in `FIX1-ci.log`.

|Failing job|Failing test / assertion|Original evidence|Class and directed change|
|---|---|---|---|
|124153748956 — bundle:02, --browser-storefront|TestBrowserStorefront → PARITY-STATES buyer routes → offline-zh-CN-1440, offline-zh-CN-390, offline-en-1440, offline-en-390: `locator.click: Timeout 30000ms exceeded`; recovery button detached|Artifact **12992061186** `shop-real-gate.log:16–55`; `parity/buyer-click-ledger.json` has 44/48 passed and precisely these four failures; original SFR01–SFR09 and OC11 ran successfully|**(b) product regression.** CartView announced repeated-read failure immediately on click, before refresh settled. The premature alert allowed the next success fixture phase while the first read was pending. Await the existing refresh before showing its result; no write/journal/provider fences changed. Add a held-read assertion that no failure alert exists while the clicked GET is pending; retain all original real clicks, exact copy, successful recovery, refresh persistence and zero-write assertions.|
|124153748986 — bundle:09, --browser-merchant-orders-ui|TestBrowserMerchantOrdersUIRealChain → W3-07B parcel merge → `suggestions 403 clears orders/detail/bulk/groups before stale read completions`: expected old sentence `This account does not have permission to read orders.`; actual signed panel. `Timed out waiting for held order poll release` follows the same failed assertion consuming the held-read deadline.|Artifact **12990484924** `playwright-parcel.log:7–42` and `results-parcel/.../error-context.md`; the separate `playwright.log` reports **45 passed**, including every PARITY-STATES admin case|**(a) replaced UI.** Require signed heading `This page cannot be opened`, exact neutral body/note and workspace link href for both suggestions/groups 403 branches, before and after stale completions. Keep 401/404 copy, real Retry clicks, all seven purge assertions, stale-response barriers, fixtures, 10s read deadline and 20s step deadline unchanged.|
|124180894791 — receipts|Exactly two non-PASS receipts: --browser-storefront (test_count 27), --browser-merchant-orders-ui (test_count 94)|Failed job log, `matrix-receipts: RED (2 problem(s))`; both bind run 41079438742/1 and merge SHA b91e125e…|Dependent on the two jobs above; no independent receipt/protocol repair.|
|124181437431 — required|Required gate depends on failed receipts|Run's raw jobs inventory: receipts failure, aggregate success, required failure|Dependent; no gate change.|

Aggregate job **124180894904 succeeded**. There is no class (c) claim: base equivalence is unproven. No job is dismissed as unrelated or flaky. This FIX1 is directed attempt 1 for the premature cart feedback and old parcel denial-copy assertions; previous relay alert-locator and shell-teardown blockers now pass in the inspected artifact. If these same assertions remain red after this batch, STOP/BLOCKED with the next exact assertion and artifact; do not attempt a third approach.

## Change and verification contract

Product fix: `CartView.tsx` clears the previous attempt's feedback, awaits the same read-only `refresh()`, then reveals the settled repeated failure. Storefront fixtures keep their existing payloads/statuses; only a labelled read-latency barrier is added to make premature feedback observable. No imports added or renamed, so no component-harness allowlist change is needed.

Acceptance gate: this PR's own Foundation and Gates at the new head, including browser-storefront (all 48 parity cases, three locales × two viewports), browser-merchant-orders-ui (45 main cases plus parcel suite), receipts and required. A held retry must show no failure alert; a completed failed retry shows the exact signed InlineError; the subsequent successful retry and reload show the empty panel; recovery sends no writes. Parcel authoritative refusals must purge private data before and after delayed responses while rendering current signed permission copy.

|Permitted local command|Exit / evidence level|
|---|---|
|`node --check tests/storefront/shop-real-gate.mjs`|0 / E1|
|`node --check tests/admin/parcel-merge.spec.ts`|0 / E1 syntax only|
|`node scripts/dev/derive-browser-ownership.mjs`, then `--check`|0 / E1, 54 modes; generated output committed|
|`bash scripts/dev/depmap.sh`, then `--check`|0 / E1, no resulting map diff|
|`python3 scripts/check_packet.py`|0 / E1, PASS_PACKET_STRUCTURE_ONLY; timestamp-only output restored|

No local tests/server/browser/PG/container/provider calls. tsc NOT_RUN (local compiler absent); Go build/vet NOT_RUN (no Go changes). GREEN, independent review, screenshot parity, density, SANDBOX/LIVE and production acceptance remain NOT_RUN/PENDING. Existing gate-adapter assertions/order, thresholds, exemption lists, contractdrift baseline and route-table dispatchers are unchanged. No migration, contract, lockfile or R2 pin authored here.

## Click ledger (CI-owned results)

|Page / control|Operation / expectation|Inspected RED / new-head result|
|---|---|---|
|Cart / signed Reload, all three locales at 1440 and 390|Real click; hold GET; no premature failure; release failed GET; exact InlineError; click again; empty state persists after reload; zero writes|Four cases fail at old head; new-head PENDING|
|Orders / parcel Retry, suggestions and groups × 401/403/404|Real click; current denial UI and full private-state purge before/after stale reads|Old 403 copy blocks suite; new-head PENDING|

## Scope, rollback and relay

FIX1 edits: `apps/storefront/components/CartView.tsx`, `tests/storefront/shop-real-gate.mjs`, `tests/admin/parcel-merge.spec.ts`, generated `output/ci-select-backend-browser/browser-ownership.json`, and `output/parity-states/{FIX1.md,FIX1-ci.log,red.log,DELIVERY.md}`. The additional parcel spec is within the same merchant-orders UI module under FIX1(a) / same-module scope preapproval; list it for integrator review in the PR body. WorkspaceFrame's header is the only authored source merge-conflict resolution. No shared-main checkout writes.

Rollback: revert this fix commit; no schema/data change. Preserve the original failed artifacts and existing relay downloads; do not stage raw ZIPs/screenshots/log trees. No runtime process started. One batch push; no merge, force push, auto-merge, status posting or deployment. Record actual Foundation/Gates workflow run IDs in the final handoff and PR body; stop polling within ~10 minutes. Integrator relays a fresh session upon completion. No evidence-only push after GREEN.

Push-sequencing deviation: the first fix commit was blocked by trailing spaces in copied evidence excerpts. A subsequent command nevertheless pushed merge-only head `107418797d74e23a2eb7f4ccb93d765fa5741809`. The evidence whitespace is now trimmed and the complete fix is committed; that intermediate head is not an implementation or acceptance candidate. Cancel its redundant queued workflows, then submit the complete fix without force-push. This deviates from the requested one-push budget and is reported explicitly; no tests/fixtures/gates were weakened.
