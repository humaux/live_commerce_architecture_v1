# PARITY-GROWTH-FIX1 — lane report (FIX1)

Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-growth` · branch `unit/parity-growth` (never pushed) · model `opencode/glm-5.3-flash-free`.
Card: PARITY-GROWTH-FIX1 (directed fix of PR #366 after run **41084050704**). Full classification table: `output/parity-growth/FIX1.md`.

## 1. Failing job → class → what this lane changed

| Failing job / test | Class | What this lane changed (file:line) |
|---|---|---|
| `gate (bundle:09)` **124178338229** `--browser-operations-ads` — `TestBrowserOperationsAds`, `getByTestId('ads-connections')` not found, expected `"9132911371291"` (`tests/admin/operations-ads.spec.ts:189`) | **(a)** the signed Figma frame moved the ad account connection to its own screen, so the older test asserted UI that no longer exists on the `/ads` list screen | `tests/admin/operations-ads.spec.ts` rewritten to the signed frame, real clicks only: `openAssets()` clicks the real "View ad account connection" button (`:182-186`, copy from `packages/i18n/src/growth.ts:29`), `adsList()` waits for the signed list screen (`:187-192`), `ads()` opens the connection screen before asserting the account (`:193-197`), every reload point re-enters it with a click (`:230`, `:255`, `:259`, `:262`), the draft-table assertion stays on the list screen (`:254`), the labels test measures both screens (`:277-284`), and `clicked()` was split into `clickedLocator()` so the new entry control is recorded in the click ledger (`:27-35`). Same intent, same coverage; no timeout, threshold, skip or fixture touched; no `apps/**` change. |
| `gate (bundle:07)` **124178338822** `--browser-cvs` — `TestBrowserTaiwanCvs/WebKit`, `getByTestId('cvs-candidate-2656')` not found at `tests/storefront/cvs-picker.mjs:387` (called from `tests/storefront/cvs-buyer.mjs:277`) | **(c)** not attributable to PR #366 | **No source change in this lane.** Proof and escalation in §3. |
| `receipts` **124208027681**, `required` **124209655416** | dependency | no separate defect; they follow the two bundle jobs. |

## 2. Evidence-retention gate completed (ADR-20261031) — required by card step 1

Card step 1 (merge `origin/r3/integration`) brings `scripts/dev/check-gates.sh:41` → `node scripts/dev/check-evidence-retention.mjs`, which the PR head never ran (the head predates that base commit). Against the merged tree it judged this PR's own `output/` additions and failed: 4 raw CI `*.log.gz` copies and 6,401,052 B > the 4 MiB per-PR budget.

- `.gitignore` — bulk Figma capture trees and raw CI failure copies are ignored, so `git add -A` cannot re-add them (`output/**/figma/`, `output/**/relay3-figma/`, `output/**/relay2-failures/`; the base already ignores `output/**/*.gz`).
- `output/parity-growth/DELIVERY.md` — the frame parity sheet cites each signed reference by **sha256** instead of linking to an untracked capture; `frame-manifest.json` keeps node id + viewport + capture path + sha256 and `relay2-figma-readback.json` keeps the Figma file key `iJ9lDdFIXgTlKkt5Tvrc0X`, the read timestamp and the per-frame context hash; the removed CI copies are cited with artifact names and sha256 (run **41054278594**; artifacts `gate-browser-meta-ads` 12978428571, `gate-browser-ads-attribution` 12978886087, `gate-browser-promotions` 12978168572). The bytes stay in git history and in GitHub Actions (90 days); the local copies stay on disk, ignored.
- Measured: `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` → **PASS (27 output/ files, 624,690 bytes)** right after this lane's change, exit 0 (before: 256 files / 6,401,052 B, exit 1). The count grew with the sibling lanes' own reports (30 files / 642,188 B at the end of this batch) and stays PASS.

## 3. bundle:07 (`--browser-cvs` WebKit) — this lane's independent analysis

Exact assertion: `expect(locator).toBeVisible() failed; Locator: getByTestId('cvs-candidate-2656'); Error: element(s) not found; Timeout 5000ms` at `tests/storefront/cvs-picker.mjs:387`, stack `exerciseCvsPicker` ← `tests/storefront/cvs-buyer.mjs:277`. Evidence: run 41084050704, job 124178338822, artifact `gate-browser-cvs` (Playwright `run.GN1qYdQ4`), `taiwan-cvs/20261030T164650.946452663/cvs-buyer.mjs.log` lines 17-52 with the failure aria snapshot (sheet open, `莱尔富` `[pressed]`, directory source rendered, query helper text visible ⇒ the typed code is not in the picker state).

1. `git diff 3c8997eb3..HEAD -- apps/storefront tests/storefront/cvs-picker.mjs tests/storefront/cvs-buyer.mjs` is **empty**: this PR contains no storefront or CVS code, and `tests/storefront/cvs-picker.mjs:379-393` is identical on the base.
2. The mechanism is base product code: a brand switch re-quotes the cart, the order form is re-keyed by quote id (`apps/storefront/components/CheckoutFlow.tsx:633`) and `OrderFlow` clears its option on every quote change (`apps/storefront/components/OrderFlow.tsx:191`), so the CVS sheet is rebuilt and the typed code is discarded — a merchant-visible defect that exists unchanged on the base.
3. The base run is green (Gates 41204331038 on `3c8997eb3`), so the card's class-(c) "identical failure on base" log line cannot be produced; the honest class is "not attributable to this PR, pre-existing race". The MOCK/chromium subtest of the same run passed the same loop (75.61 s), i.e. it is timing-dependent.
4. The durable fix therefore sits in product code under `apps/storefront/**`, which this backend-only lane may not touch ("do not change anything under apps/**"); weakening the CVS test (retry, longer timeout, dropped assertion) is forbidden by AGENTS.md and by the card.

**What the tree now carries (sibling lane, commit `642d3f0dd`):** a test-side settle in `tests/storefront/cvs-picker.mjs:380-386` — after each real brand click it awaits the existing `POST /api/buyer/quotes` response and the pressed state before filling the code. That is a candidate, not a proven closure: by this lane's reading the `aria-pressed` gate already implies the rebuilt sheet, so the added wait may not remove the race, and only the next CI run can judge it. **If bundle:07 is still red after the next run, escalate to a storefront lane** — the shared helper must keep the typed code when the sheet is rebuilt (product fix under `apps/storefront/**`).

## 4. Concurrent lanes in the same worktree (must be reconciled by the integrator)

Sibling lanes (PARITY-GROWTH-FIX2, PARITY-GROWTH-FIX1-R2 and a further PARITY-GROWTH-FIX1-R4 round) wrote **the same worktree and branch** while this lane worked. Two of them are in the branch history:

- `e9a0e8994` (PARITY-GROWTH-FIX2, "align operations ads connection assertions to signed frame and untrack raw captures") — carries this lane's class-(a) spec rewrite (the file was already staged in the shared index), an `apps/admin/components/Ads.tsx` testid for the asset screen entry, the regenerated browser ownership, that lane's two reports and the untracking of the raw captures. This lane's rewrite does not depend on that testid (it locates the control by role + signed copy), so the tree stays consistent either way.
- `642d3f0dd` (PARITY-GROWTH-FIX1-R2) — the bundle:07 test-side settle (`tests/storefront/cvs-picker.mjs:380-386`), the refreshed browser ownership and `output/parity-growth-fix1-r2/LANE-REPORT.md`; it also wrote the card's `output/parity-growth/FIX1.md` table (its version, "FIX1 R4"), which is the current copy of that shared deliverable, while this lane's independent classification and proof stay in this report.
- This lane committed only its own paths: `.gitignore`, `output/parity-growth/DELIVERY.md` and `output/parity-growth-fix1-r1/LANE-REPORT.md` (commits `dacdbe65a`, `12242c3f0`), plus the card table it wrote before the sibling's version replaced it.
- The R4 round was still in flight when this lane finished (its `output/parity-growth-fix1-r4/` report and its `DELIVERY.md` / `FIX1.md` edits were uncommitted); this lane left them untouched and did not commit them.

## 5. Verification (static only — owner rule: every test runs in GitHub CI)

| Command | Result |
|---|---|
| `node scripts/dev/derive-browser-ownership.mjs` | wrote `output/ci-select-backend-browser/browser-ownership.json` (54 modes) |
| `node scripts/dev/derive-browser-ownership.mjs --check` | exit 0 (no ownership drift) |
| `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` | **PASS** (27 output/ files, 624,690 bytes), exit 0 |
| `gofmt -l .` | no output, exit 0 |
| `git diff --check` | clean (no whitespace damage) |

NOT_RUN (deliberately, owner rule — no tests, builds, servers or browsers in this lane): `go test -race ./...`, the Playwright suites (`--browser-operations-ads`, `--browser-cvs`), `pnpm build:*`, `pnpm typecheck:*` (a build command), the browser gates themselves. The class-(a) rewrite is therefore **not** locally proven; it is proven by the next CI run.

## 6. Risks and open items

- **bundle:07 is a candidate, not a proven closure** (§3): the sibling lane's test-side settle is committed, but this lane's analysis says the root cause is product-side (the checkout form re-creates the CVS sheet when the cart is re-quoted). If the next run still shows `--browser-cvs` red, escalate to a storefront lane — the shared helper must keep the typed code when the sheet is rebuilt.
- The class-(a) rewrite keeps every original assertion (permissions hide mutation controls, feed copy, unbind pause-first/history refusals, in-flight wording in every locale, draft history after refresh) and adds one evidence screenshot per locale/viewport; the harness minimum of 12 hashed screenshots stays satisfied (18).
- Nothing under `apps/**`, `internal/**`, `migrations/**`, `scripts/dev/contractdrift/**`, `.github/**`, `go.mod`, `go.sum`, `pnpm-lock.yaml` was changed by this lane; no test was deleted, skipped or loosened; no secret or real buyer PII was written; no CI artifact or job log was committed.

## 7. Handoff

WAIT_RUN: the branch carries the class-(a) rewrite of `tests/admin/operations-ads.spec.ts` (committed in `e9a0e8994`), the bundle:07 test-side settle (committed in `642d3f0dd`), this lane's ADR-20261031 evidence-retention completion (`.gitignore` + the sha256 citations in `output/parity-growth/DELIVERY.md`, commit `dacdbe65a`) and the card's classification table (`output/parity-growth/FIX1.md`). Nothing was pushed by this lane. The integrator pushes; the next Gates/Foundation run on the PR head decides `--browser-operations-ads` (expected PASS) and `--browser-cvs` (candidate — see §3; escalate to a storefront lane if it is still red).

STATUS: DONE
