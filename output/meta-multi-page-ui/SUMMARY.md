# meta-multi-page-ui — implementation delivered, backend gate BLOCKED

## Scope and authority

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/meta-multi-page-ui`
- Branch: `unit/meta-multi-page-ui`
- Observed starting SHA: `152a11b469c17700a306fa80c6fa7d6c1af0b230` (backend `452556d` plus independent K3 tests).
- Final tested source: `9177d4e016d8f42927f5bb23b089032ed5b2d4cb`. Later commits contain evidence only.
- Root task: `f97c8f5e-16b7-492f-9419-98073960c777`, agent `codex-meta-multi-page-ui`. Root is sole writer; role UI implementation/integration. Exact runtime model and reasoning setting are not exposed: **UNKNOWN**, not inferred.
- Write paths: `apps/admin/components/{MetaConnect,SettingsWizard,StudioClaims}.tsx`, `claims.css`, `settings.css`, `apps/admin/lib/meta-connect-{model,client,copy}.ts`, `meta-page-source.ts`, `tests/admin/meta-connect*.ts`, this evidence directory.
- **No Go/SQL/deploy changes, push, merge, production access or credential access.** No other worktree was edited. Shared PG/browser tests used the machine lock and the scripts' own locks; fixture teardown completed.
- Existing shell retained: this branch does not include W0, so it was not merged or recreated here.

## Requested work

| Item | Status | Implementation / evidence |
| --- | --- | --- |
| 1. Settings > Facebook multi-Page list | PASS — UI/REAL_PG with fake Meta | Strict `pages[]` parser; Page name/ID, IG, access status, last event, connection/expiry times. All timestamps use the existing Taipei formatter and show the timezone. `N / 10`, cap explanation and disabled add button. |
| 1. Disconnect / reauthorize / add | PASS — UI, with backend limitation below | Each row disconnects only its `page_id`, with named confirmation explaining that its comments no longer create orders. Global **choose a Page to reauthorize** action avoids implying that B is targeted while OAuth defaults to A. Add uses the same authorization command. |
| 2. Studio selector grouped by Page | PASS for list, selection and binding B; BLOCKED for rebind after B disconnect | Every connected Page is listed in an `optgroup`. Reauth-required Pages are shown but disabled. A bare numeric Facebook post/live ID is qualified using the selected Page. Mismatched Page IDs, hostile URLs and ambiguous named links are rejected without silently repointing them. After B disconnect, A remains listed/selectable; saving a replacement source fails in Go/SQL — see B1. |
| 3. Keys and errors | PASS for client/BFF contract; server receipt idempotency NOT DELIVERED | All three Meta POST paths send `Idempotency-Key`; Studio PUT retains the existing command mechanism. `cap_exceeded` and `not_found` have clear three-locale messages. BFF's existing bounded error-code pass-through needs no second implementation. Start UNKNOWN retains the same key/body; pick/disconnect UNKNOWN do **GET-only** recovery, never blindly re-POST. See B2. |
| 4. Singleton retirement | PASS | Removed the singleton status union/parser and empty disconnect body. One status read, one pick command, one disconnect command; no legacy parallel writer. Settings mounts the card keyed by store ID. |

Instagram account information is displayed beside its Facebook Page. This unit's explicit Page selector qualifies Facebook post/live IDs. It does not invent an Instagram asset-ID input unsupported by the backend; multiple IG media binding disambiguation remains a separate backend contract issue.

## Audit and safety corrections

- Audit-first: the old card assumed one Page, sent `{}` to disconnect and formatted time in the browser's timezone. Repeated Page rows now share one rendering/command path; no unrelated global controls were added.
- `frontend-architect` and Impeccable hardening guided scoped state/API separation, honest unavailable/UNKNOWN states and touch/layout checks; Playwright was used for executable evidence.
- Independent review found and closed three UI hazards: unknown successful start body discarded its key; per-row reauthorization falsely implied a target; unknown pick could use a missing status baseline. The final submit handler **and** button require a completed same-store status read.
- An old active Page and unchanged `connected_at` never prove that a lost reauthorization succeeded. A failed pick remains locked/UNKNOWN while status is reread.
- Reading a newly connected target Page or a changed connection timestamp confirms the desired connection state, not a replay receipt. Reading a disconnected Page as absent confirms the desired disconnect state.
- Existing BFF tenant/session/CSRF guards, strict DTO parsing, permission denials, OAuth state/cookie and open-redirect assertions remain in place.

## Final required gates — exact commands and exits

All commands below ran from this worktree on **9177d4e0**, with no source edits during execution.

| Command | Exit | Result / evidence |
| --- | ---: | --- |
| `bash scripts/dev/test-node.sh` | 0 | 282 passed, 0 failed. `test-node-final-source.log`; optional R04 binary suite explicitly NOT_RUN. |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc-final-source.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 55 modes documented, every tracked test file covered. `check-gates-final-source.log` |
| `bash scripts/dev/test-local.sh --browser-meta-connect` | **1** | Original suite PASS; independent suite 12 PASS / 1 FAIL on backend B1. `browser-meta-connect-final-source.log`, `meta-connect-independent-final.log`, `meta-connect-legacy-final.log`. |
| `bash scripts/dev/test-local.sh --browser-studio-ui` | 0 | Signed MOCK identity + real UI/BFF/Go/PG + MOCK worker. `browser-studio-ui-final-source.log`. |
| `bash scripts/dev/test-local.sh --browser-live-claims` | 0 | KC16, 18 cases, three locales, real admin/storefront/BFF/Go/PG, MOCK manual ingress. `browser-live-claims-final-source.log`. |

Other checks: `git diff --check` exit 0. Initial new strict DTO/copy assertions failed before implementation (`model-red.log`, exit 1), then passed (`model-green.log`, exit 0); final full Node suite includes those assertions and Page/source negatives.

### Browser evidence runs

- Final independent Meta run: `output/playwright/meta-connect-gate/20261002T150204.927045000` — **12 passed, 1 failed**.
- Final legacy Meta run: `output/playwright/meta-connect/20261002T150231.126074000` — PASS.
- Final Studio: `output/playwright/studio-ui-20261002T150323.376951000` — PASS.
- Final live claims: `output/playwright/live-claims-2177196183` — PASS.
- 21 final PNGs copied into this directory: `multi-pages-*`, `multi-page-b-*`, `multi-studio-*` (3 locales × 2 sizes), plus three `MOCK-cap-ten-*` mobile images.
- `screenshots.json` records source SHA, original run, locale, evidence tier, dimensions and SHA-256. All requested desktop images are **1586×992**, mobile **390×844**, CSS pixels; dimension mismatches: 0.
- The requested Settings card and Studio source section are captured at their section headings, not the overall page top. Long content continuing vertically is intentional; browser assertions check no horizontal overflow. New Page select and Settings buttons have minimum 44px touch height.
- `red/backend-rebind-failure.png` is intentionally retained failure evidence, not a passing screenshot.
- Copied log files have terminal trailing whitespace/carriage returns normalized for Git; assertions, outputs and outcomes are unchanged. Original browser run artifacts remain at the paths above.

## BLOCKED — B1: disconnected old source cannot be retired during rebind

Severity **P1, backend owner action required**. The stronger end-to-end assertion is retained, not skipped or weakened.

Reproduction in the final real local stack:

1. Connect A and B through actual UI/BFF/Go/PG with fake Graph.
2. Both Pages appear in Settings and Studio; selecting B + bare post ID binds `B_223456789012345` successfully.
3. Disconnect B using body `{page_id: B}`. Even when its successful response is lost, UI rereads status; B disappears and A remains.
4. Select A, submit `A_323456789012345` on the same scene with the current source version.
5. Go returns **409 `binding_missing`**; the scene still shows its old B source. The final gate correctly fails.

Verified cause: `internal/claims/claim_source.go` retires the current active source through `putSource` before rebinding (lines 374–380 at this base). `live.put_claim_source` in `migrations/0064_meta_claims_intake.sql` unconditionally requires an enabled route for the old asset, even when deactivating it (lines 747–753). B's route was disabled by disconnect, so that retirement fails. The request's selected A ID is correct; this is not a UI selector or amount/tenant problem.

Required backend adjudication: allow safe retirement/rebind of a disconnected prior source while preserving tenant/store, version and ownership checks, or define the intended alternate business flow. **Do not weaken the browser assertion.** Re-run the existing full `--browser-meta-connect` after the backend fix is integrated.

## Backend constraint — B2: keys do not currently give pick/disconnect replay receipts

`internal/httpapi/meta_connect.go` requires the header, but `Pick` and `Disconnect` are not passed its value and do not provide result replay keyed by that header. Start does. Merely sending a key must not be reported as end-to-end deduplication.

This UI limits recovery for pick/disconnect to status reads. A fully unresolved attempt remains locked in the mounted UI; cross-refresh durable command recovery is **not** claimed. A backend receipt/reconciliation contract is needed for full recovery of unresolved attempts. Existing separately reported backend disconnect/reconnect unsubscribe race also remains outside this UI unit; integrator must incorporate its backend fix before release.

## Independent review / visible remaining P2

- Read-only fixture explorer: existing fake Graph A/B + Studio fixture sufficient; task `3f5e1947-ea66-4273-b60f-e5eace09533f`. No Go changes required for our test extensions.
- Read-only security reviewer: final source recheck of `6f6d4c58`, no remaining/new UI P0/P1; memory `14c85d45-c76a-4b07-af8d-21265cb61a59`, task `8b992033-1be2-4dd6-9413-2f05b0360682`. Reviewer did not independently run browsers; root ran the commands above. `9177d4e0` changes screenshot framing only.
- Read-only visual reviewer: 21 images checked; one **P2** in `multi-studio-en-desktop.png`: existing Offers form compresses Keyword/Product/SKU, clips “Add offer” and wraps “Add to library”. Its grid and controls were not changed by this unit; retain as an adjacent Studio layout follow-up, not a claimed clean whole-page visual pass. Memory `4759e3c3-56de-42aa-8ca3-1484b5783d69`.
- Subagents were readonly, reused scoped roles, no recursive delegation; exact model/reasoning unavailable and recorded UNKNOWN. Root owns all changed files.

## Assertion and attempt ledger

- No security/permission/CSRF/isolation/OAuth/source assertion was deleted or relaxed.
- Exactly one obsolete feature restriction changed: the old test asserted no reauthorize action for a still-active token. R5 explicitly requires reauthorization/add, so it now asserts both are enabled. The old initial-connect action is still absent after connection. This is an explicit contract update, not a safety threshold change.
- First browser run failed because the new direct `APIRequestContext` read did not use the browser's secure-cookie session; changed it to same-origin browser fetch, retaining the required 200 and DTO assertions.
- Next run exposed backend B1. It was not modified. The case was moved last so other independent assertions could finish and generate evidence.
- The first pure MOCK cap fixture omitted the existing mandatory `Cache-Control: private, no-store` read header. Fixed the fixture to match the real BFF; the read guard was not relaxed.
- Final runs still retain B1 as the only Meta failure. Repeated runs after it were for frontend baseline/cap counterexamples and final screenshot framing, not attempts to bypass the backend failure. Earlier logs stay in this directory for traceability.

## Commits

| SHA | Logical change |
| --- | --- |
| `130b00a3` | Multi-Page DTO/client/UI, explicit Studio Page selection, pure contract tests |
| `c530c4a6` | Multi-Page browser acceptance, UNKNOWN response loss, cap/error boundary cases |
| `67edf560` | Browser-session status read in the test driver |
| `4c973fc3` | Require loaded status baseline; delayed-read/unchanged-old-state counterexample |
| `6f6d4c58` | Recovery controls, 44px Page selector and accurate no-store MOCK responses |
| `9177d4e0` | Frame evidence from relevant section headings; final tested source |

All authored commits carry `Co-Authored-By: Codex <noreply@openai.com>`. This file and logs/screenshots are committed separately as evidence.

## NOT_RUN / not claimed

- Real Meta SANDBOX/LIVE OAuth, subscriptions, comments, messages, App Review, customer data or real account mutations.
- Production deployment, merge, push, Go/SQL edits, live payments/refunds/logistics.
- Optional `tests/media/r04-input-runner.test.mjs` (pinned LiveKit binary unset, explicitly reported by test-node).
- New backend receipt/idempotency acceptance, or a fix to B1/unsubscribe race.
- Multiple Instagram assets' explicit source qualification; backend accepts platform hints but no IG asset selector field.
- Entire SaaS release acceptance or an all-green Meta browser gate. **This handoff is not merge approval.**

## Next owner action

1. Backend owner resolves B1 and the outstanding backend recovery/race constraints without changing this unit's assertions.
2. Integrator brings the backend correction onto the unit branch and reruns `--browser-meta-connect`, then any required independent final gates.
3. Adjacent Studio owner handles the inherited English Offers layout P2.
4. Only after those decisions and gate evidence should integration/release approval be considered.
