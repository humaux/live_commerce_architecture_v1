# LC-U3 create-order drawer delivery

## Current batch — trunk merge + W3-U2 blocklist warning (2026-10-09)

- Source commit **`3a0f6aa1b36c246127986eca38342c0516544935`**, parents old K3-reviewed `f515dcdb` and trunk `4ff99766`. Branch `unit/lc-u3-create-order-drawer`; own worktree unchanged. No push/deployment. Parent model/effort is not exposed by this runtime; read-only explorers explicitly used `gpt-6.1-sol/high`.
- Trunk registry is byte-identical (82 modes, `--list` exit0), along with the 13 reviewed LC-U2a seams/tests. Merge preserves trunk privacy/cursor/parcel/import routes and reapplies the six U3 CommentStream drawer hooks.
- Warning consumes the existing **W3-05B boolean-only GET**, no contract change: regular BFF `/live-sessions/{sid}/claims/blocklist/check?bundle_id=...`, exact canonical query, HTTP200 `{restricted:boolean}`, no-store/no-referrer, bounded response. The SID comes from the server A13 claim snapshot or the current CommentStream and is only a store-level path prefix. Every known A15 bundle is checked under a common18s budget; auth failure always clears private state, non-auth lookup failure cannot claim clear. Three locales show restricted/unknown guidance; restriction never changes Quote, body, key, permission or submit eligibility.
- Own functional write paths: `apps/admin/components/{CreateOrderDrawer,BuyerPanel,CommentStream}.tsx`, `apps/admin/lib/{claims-request,create-order-client,create-order-copy}.ts`, `apps/admin/proxy.ts`, the root store catchall BFF, `tests/admin/create-order-{client,bff}.test.ts`, `tests/admin/create-order-drawer-gate.mjs`, `tests/foundation/browser_create_order_drawer_test.go`, and this unit's curated delivery/evidence. Merge-only paths preserve trunk versions. No product Go, SQL, migration, schema, lockfile or W3-U2 UI edits.
- Real fixture uses existing signed Meta ingress/consumer/intake in the trade store, validates APPLIED/ACCEPTED/facebook/quantity2, and executes real A14 link + restriction POST. Cleanup is limited to its six sessions/restrictions; first consumer stops before the differently-keyed comment harness. New drawer runs use ignored `output/playwright/create-order-drawer-*`.

| Local command | Exit/result | Raw evidence in `output/playwright/lc-u3-merge-validation/` |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0,1295 tests | `node-final.{log,status.json}` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `tsc-final.{log,status.json}` |
| `bash scripts/dev/check-gates.sh` | 0,82 modes/headers | `gates-final.{log,status.json}` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `vet-final.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 0,legacy10 +drawer25,PG14/14 | `manual-order-green-ignored.{log,status.json}` |
| Same mode with `LC_DRAWER_CALIBRATION=new-key-on-retry` | 1 expected,only `DU3-single-order-en-1440`,PG15/14 | `manual-new-key-calibration.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-live-console` | 0 | `live-console-green.{log,status.json}` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-inbox` | 0,real chain/I11 | `inbox-green.{log,status.json}` |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | mode0,24 cases +role/store/axe | `admin-shell-green.{log,status.json}` |

- E3 evidence binds every final run to unchanged source hash **`3a8ceac72784de4ee5e958501f299245e3f95387af60758e2a9c271c6d32a137`**, committed verbatim in `3a0f6aa1`. Curated metadata: `merge-warning-evidence.json`; actual browser/Go/PG with synthetic data and MOCK identity/Meta only. Independent read-only merge and source/evidence review found no newP0/P1; integrator's K3 delta re-review remains required.
- Warning browser: `output/playwright/create-order-drawer-3722851862/{result,click-ledger,pg-readback}.json` has25/25, six real BuyerPanel warning clicks (en/zh-TW/zh-CN×1440/390), seven same-key/body one-order/receipt retries, and PG14/14. Calibration root `create-order-drawer-4163076078` has25 cases and exactly the intentional changed-key failure: same body, two request orders/receipts, replay201, PG15/14.
- New BFF gate non-vacuity: remove only the GET grammar for the actual authenticated root handler → **RED405 vs200 (exit1)**, restore exact bytes → GREEN0. `blocklist-root-bff-{red,green}-final.log`, `blocklist-bff-red-green.json`. Initial tools-route probe and manual-bundle422 are setup failures, not causal product red proofs. One consumer wait also failed without a diagnostic; no unproven root cause is claimed. All final normal/calibration runs completed without worker/cleanup errors. Early submit-enable assertion was moved to the valid payment/Quote state; it remains required before the real201 submit.
- Trunk's old Admin shell mode overwrote tracked evidence; the protection wrapper correctly exited99 despite mode0. All88 generated files were preserved under `admin-shell-evidence/`, then historical output restored byte-for-byte to hash `9e02b5a43e89f1c334d1f6b3721f0366d4cda6db5db9f4bf68241ed92b4e8aac`; source stayed unchanged and git was clean. See `admin-shell-evidence-restore.json`. Earlier untracked drawer runs are preserved in `legacy-layout/`; no historical screenshots/logs are committed anew.

### CI gates

Changed browser spec/mode remains only `--browser-manual-order`: `create-order-drawer-gate.mjs` and its Go fixture; no added mode. Local hosts rerun: live-console, inbox, admin-shell. Run manual-order and its expected-red calibration in CI; retain **click-sweep and visual-lint**, NOT_RUN locally per the frozen brief. `node scripts/dev/pr-modes.mjs origin/r3/integration` selects53 modes due to shared BFF/proxy/forms; the complete exact list is in `merge-warning-evidence.json.pr_modes`. Remaining selected modes, full foundation/G07, SANDBOX/LIVE providers and deployment are NOT_RUN locally. Integrator re-reviews this delta and pushes.


## Prior addendum — ManualOrder P1 (4212512394)

- Implementation `d0422a6c`; real regressions `0d91f236`, `778de008`; base previous delivery `6aabe6b6`. This addendum supersedes the original claim that ManualOrder submission behavior is unchanged.
- Root cause: editing the live draft after UNKNOWN regenerated its request key/body. The real baseline `addendum-edit-unknown-red.log` failed only `DU3-manual-unknown-draft-edit`: two attempts, different key and body, HTTP201, two request-specific PG orders/receipts (store total15 instead of14).
- Shared `OrderAttempt` now retains its immutable snapshot after both ambiguous and definitive responses. `finish()` only records definitive settlement; only the explicit `startNew()` action clears the receipt for a fresh key. ManualOrder keeps editable draft fields separate; retries ignore draft edits and use the original request. Drawer keeps editing blocked for unresolved attempts. Both submit handlers and buttons reject a settled receipt until the merchant clicks the localized new-attempt control.
- ManualOrder HTTP200 replay accepts the server-redacted configured/null buyer link without treating a successful replay as UNKNOWN; HTTP201 remains strict. Existing regeneration recovers that link using the original submitted locale. The default result parser remains strict.
- **Final real browser GREEN:** `addendum-edit-unknown-green.log`, `browser/20261008T154854.154170000/result.json`: 25 cases, PG14/14, legacy10/10. The edited ManualOrder retry has same key/body, HTTP200, PG delta1, exact one order and receipt. Both real422 refusal subchecks prove editing cannot enable submit, explicit new-attempt enables it, and clicking that action sends no POST and creates no order.
- **Calibration:** `addendum-calibration-red.log` exits1 only for `DU3-single-order-en-1440`; all25 cases run, PG15 instead of14. The ManualOrder edited-draft regression remains GREEN during this drawer fault.
- **Final local commands:** `bash scripts/dev/test-node.sh`, `bash scripts/dev/check-gates.sh`, `pnpm --dir apps/admin exec tsc --noEmit -p .`, `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation`, `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` all exit0. `LC_DRAWER_CALIBRATION=new-key-on-retry GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` exits1 as required. Evidence: `addendum-{node,gates,typecheck,vet,edit-unknown-green,calibration-red}.{log,status.json}`.
- All final commands share unchanged source SHA-256 `091a4f21bac71315adf405441d5bb11e23520b26fd56342ed73a621dc0dcd7ea`, committed verbatim in `d0422a6c`. Node lifecycle red→green: `addendum-lifecycle-{red,green}.log`.
- E3, real Chromium/Next/BFF/Go/PG with synthetic data and MOCK identity/provider. Read-only source spotcheck found no additional money/state issue; K3 independent acceptance remains pending. Mode remains `--browser-manual-order`; no new mode, product Go or contract change.
- Test author configured `gpt-6.1-sol/high`, separate `unit/lc-u3-retry-tests` worktree, own paths limited to browser driver and Go fixture. Root implemented/reviewed product and ran every runtime gate. Temporary test worktree archived then removed; delivery worktree retained. No push.

## Original delivery evidence (historical, before this addendum)


- Branch: `unit/lc-u3-create-order-drawer`. Base: LC-U2a `9ce09803`; frozen brief `ba2bab32`. No trunk merge, push, PR or deployment performed.
- Tested implementation: `faac7bf2` (preceding integration `142a58c5`, API `97b1243d`, shared form `d00d04c7`). All final gates share source SHA-256 `16e8ac45852239ae02f80174697bfc84f96003e63459dd38e705124badb5e988`; each `*.status.json` records command, base HEAD, timeout, elapsed time, source hash and unchanged-source check. The five then-uncommitted changes were committed verbatim as `faac7bf2`.
- Implementer: Codex-3, GPT-6 runtime family (exact parent deployment/effort not exposed). Child explorers and writers configured `gpt-6.1-sol`, high effort except browser mapping medium. Child worktrees were isolated; no recursive delegation.
- Contract/interface changes: none; consumes live-console-v1 §5/§6/A15/A16 and remaining-quantity amendment. UI/BFF and Go browser fixtures only; no product Go, migration, schema or lockfile change.

## Implementation

- `apps/admin/components/ManualOrderFormFields.tsx` and `ManualOrderItemPicker.tsx` share existing fields. Legacy ManualOrder submission, result and link behavior remain intact.
- `CreateOrderDrawer.tsx:79` opens a native modal from BuyerPanel or a claimed comment. Only A15 explicit customer links prefill contact/delivery; original claim facts retain individual remaining quantities even when request SKU quantities aggregate.
- A15-ineligible/live-permission warning precedes confirmation. Catalog unit references are informational; the server Quote supplies all actual prices and totals. No client price fields, total computation or identity inference.
- Exact tools BFF resources: GET `inbox/order-prefill` with one canonical selector and POST `orders/for-buyer` with closed body/header rules. Tests execute real Request/auth/origin/CSRF/store helpers, replacing only upstream network.
- `create-order-attempt.ts:6` freezes request bytes/key; UNKNOWN locks edits and replays that same receipt. A coarse boolean guard survives hide/unmount without storing buyer data, body, key or link. First-dispatch 401/403 also retains the guard because A16 can reauthorize DM after order commit. Session loss clears drawer and parent private state.
- Result copy covers every allowed coded error in three locales; duplicate bundle gives only the existing-order action. A9 conversation binding must exactly match B1 `dm_session` capability. Payment URL remains a transient ref and goes only to native clipboard; replay-redacted URLs use the existing stable-key regenerate-link endpoint.

## Red → green evidence

- `api/red.log`: missing model before implementation; `api/green.log`: real-seam focused cases pass. Existing manual parser remains strict; only A16 accepts replay-redacted configured links.
- `attempt-red.log` → `attempt-green.log`; `postcommit-auth-red.log` → `postcommit-auth-green.log`: immutable retries, storage fail-closed and auth-after-commit protection.
- `copy-red.log` → `copy-green.log`: all coded errors/reasons and eligibility warning, all locales.
- `browser-missing-drawer-red.log`: legacy flow reached new test; absent drawer generated no A15 request. Initial setup failure is separately retained in `browser-baseline-red.log` (test fixture log-stream fd not ready; fixed by awaiting open). Neither is represented as a product success.
- `browser-drawer-green-1.log`: real creation, copy and duplicate flow passed; missing address control test IDs exposed and corrected without changing assertions.
- Normal final `browser-drawer-green-2.log`: **24/24 matrix cases; PG 13/13 orders**, plus direct signed-ingress comment trigger and six session-boundary purges; unchanged legacy **10/10** cases. Details/screenshots: `browser/20261008T152835.002287000/`.
- `browser-new-key-calibration-red.log`: exit **1**, exactly `DU3-single-order-en-1440` fails; all 24 cases run. Mutation preserves body but forwards a new retry key at the real HTTP edge: two attempts, two request-specific PG orders/receipts, replay status 201, PG total 14 instead of 13. Details: `browser/20261008T152955.378435000/result.json`. Mutation is opt-in test-only, no product calibration branch.

## Local gates

| Command | Exit | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` | 0 | `root-node.log`, `.status.json` |
| `bash scripts/dev/check-gates.sh` | 0 | `root-gates.log`, `.status.json` |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | 0 | `root-typecheck.log`, `.status.json` (equivalent pnpm --dir invocation) |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `root-browser-vet.log`, `.status.json` |
| `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 0 | `browser-drawer-green-2.log`, `.status.json` |
| `LC_DRAWER_CALIBRATION=new-key-on-retry GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-manual-order` | 1 expected | `browser-new-key-calibration-red.log`, `.status.json` |

Evidence class: **E3, REAL_PG + real Chromium/Next/BFF/Go**, synthetic fixtures and signed MOCK OIDC/Meta only. Component/Node tests are MOCK, not substitutes for browser evidence. Representative 1440/390 screenshots inspected; full-page captures include the underlying page below the viewport-height native dialog. Independent source spotcheck corrected the selector/privacy/remaining-quantity findings; K3 acceptance remains pending.

## Modes/specs and CI gates

Changed browser mode: **`--browser-manual-order`**, `TestBrowserManualOrderLink` in `tests/foundation/browser_manual_order_link_test.go`, new `browser_create_order_drawer_test.go`, new `tests/admin/create-order-drawer-gate.mjs`. No new mode or fixture-only UI route. Existing legacy spec stays intact.

Integrator CI: `--browser-manual-order` plus its new-key calibration; `--browser-live-console`, `--browser-inbox`, `--browser-admin-shell`, `--browser-click-sweep`, `--browser-visual-lint` for affected hosts/shared fields. These additional modes were **NOT_RUN locally**; required manual-order mode was run at the owner's explicit request despite the general CI-only browser default. G07/full foundation, real provider sends, SANDBOX, LIVE and deployment NOT_RUN.

## Limits and handoff

- More than five prefill bundles is refused without truncation; open a single claim bundle.
- After an unresolved request loses private memory, no status-by-key endpoint exists. The current store/auth scope stays blocked; no new key is silently generated and no private retry journal is persisted. External reconciliation/session recovery is an operational limitation for review.
- Direct comment/bare bundle cannot infer a messaging identity and therefore uses catalog pricing with a visible warning.
- K3 deep review, push/PR and merge belong to the integrator. Merge trunk only after the integrator confirms LC-U2a merge. No migration, grant or schema action requested.

Cleanup: owned form/API/browser/UI temporary worktrees were archived under the main checkout `output/lc-u3-create-order-drawer/worker-archives/` and removed; their commits remain. Requested delivery worktree remains. All owned gate runners ended and test-local cleaned its fixture containers/ports.
