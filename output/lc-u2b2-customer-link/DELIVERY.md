<!-- Purpose: hand off the LC-U2b2 UI/BFF checkpoint with measured gates and the remaining positive-browser fixture gap.
Depends on: base abe4714f, frozen live-console-v1 A9/A13/A14 and LC-B3b, current Node/browser receipts.
Used by: integrator scope ruling, K3 review and later complete acceptance; not a declaration that all unit gates passed. -->
# LC-U2b2 customer link / BuyerPanel checkpoint

**2026-10-08 update: test-fixture scope is APPROVED.** The new test-only extension below is uncommitted and E1/structure-ready. Full acceptance is still BLOCKED by this session’s runtime permissions (Git metadata read-only, loopback listen EPERM, Docker socket denied). No product Go, SQL, migration or OpenAPI changed. Historical green results below apply only to their recorded earlier source.

- Branch: `unit/lc-u2b2-customer-link`; base `abe4714fb2026df609590a57da6862a0ff6e5b24`.
- Green implementation commit: `21fce0f4`; tested browser source: `9edb011e765446dea2f90d66aa724fa819880bc3`.
- Final documentation SHA/source manifest: main checkout `output/lc-u2b2-customer-link/final-receipt.json` after commit.
- Root: Codex UI/BFF implementer. Read-only explorer/high and security_reviewer/high; exact deployed model identifier unavailable. No child code writes or recursive delegation. K3/PR/push belong to the integrator.

## Implemented

BuyerPanel reads the CAS version from A9, never A13 or inferred identity. It reuses the existing customer-list read with an empty search query and bounded pages; only explicit picker choices are submitted. Names stay in memory and never enter search URLs. A14 link/unlink uses the original immutable key/body on uncertain retry; 409 refreshes the actual winning customer/name and A9 version while keeping the merchant's pending choice. A14 404 and authorization loss clear private data and obsolete completions. Both reply and customer-read permission hints are required; the Go server remains authoritative.

A13 now shows claim keyword/quantity/session, the supplied total in TWD, purchase ordinal, automatic-reply state and authorized orders. Omitted orders have distinct server-withheld copy. Explicit linked customer names come from the existing scoped customer read. No name matching or owner_id inference. Reply capability matches A9 binding_id exactly; null/missing binding is unavailable, unrelated Pages grant no authority, and review_required gets its own badge.

The A14 BFF route/method/exact body allowlist already existed. It is reused unchanged; additional real Request/real route/auth seam tests assert method/body/origin/CSRF refusal, closed 409/404 errors and no private upstream details. No other BFF route was widened.

## Actual local evidence

All raw logs/status JSON and retained browser evidence are in the **main checkout** `output/lc-u2b2-customer-link/`. PG/browser modes ran serially, pinned to the stated source SHA. Local browser runs are explicitly authorized by this task, overriding the old preamble's CI-only default.

| Command | Exit | Scope/result |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/inbox-customer-link.test.ts` | 1 → 0 | Initial 0 pass/9 fail → green; additional session-boundary and CAS-winner counterexamples each 9 pass/1 fail before their fixes |
| Focused new/BFF/review/recovery Node suites | 0 | `step1-green-focused`: 60 pass, 0 fail; exact command in status JSON |
| `bash scripts/dev/test-node.sh` | 0 | `step1-green-node` and `step2-node`: 890 pass, 0 fail |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `step1-green-types` |
| Strict tsc, inbox-ui.spec.ts + inbox-bundle-ui.spec.ts | 0 | exact args in `spec-types.status.json` |
| `bash scripts/dev/check-gates.sh` | 0 | `step1-green-gates`, `step2-gates` |
| `bash scripts/dev/test-local.sh --browser-inbox` | 0 | `browser-normal`: existing 13 browser cases + real Go/PG harness; includes new customer-read denial at 3 locales × 2 widths |
| `LC_INBOX_CALIBRATION=retain-thread bash scripts/dev/test-local.sh --browser-inbox` | 1 expected | Same 9edb011e SHA: exactly INU05 hidden thread retains private DM; 12 other browser cases pass; `inu05-verdict.json` |

Evidence class: E3 only for the tested Node/component/BFF and current BROWSER signed MOCK/REAL_PG scope. The normal 13-case suite does **not** include the still-missing positive A14/full-A13 seed acceptance. It must not be represented as complete LC-U2b2 browser acceptance.

The read-only privacy reviewer found one P1: plain sessionBoundary errors were classified as unavailable503, leaving prior customer data visible. Both boundary checks now explicitly become unauthorized401; the real cookie-change-during-JSON counterexample verifies teardown/callback. Reviewer closure: Humaux `451206a2-cf52-4868-99c0-d872bf272276`, source/receipt review only, independent runtime NOT_RUN. Root also added a CAS-winner counterexample so refreshed version cannot unlock actions while the old linked name remains displayed.

Failure records are preserved: the first BFF test oracle expected a code-only error, while real auth.ts emits a safe standardized envelope with status-specific messages. The oracle now checks its exact safe keys and excludes the synthetic private upstream value; no shared helper was mocked or changed. An intermediate bundle-only regression was fixed by keeping A14 controls absent without a conversation. Old missing-version/advisory fixtures were updated to actual A9 authority while retaining their privacy/refusal assertions. Failed attempts are not counted as green.

## Remaining / CI gates

- New real signed actor/full A13 test fixture and six positive browser variants are implemented but runtime NOT_RUN; see the current fixture extension below. Scope approval is resolved, environment permissions remain blocked.
- CI: rerun `--browser-inbox` normal + `LC_INBOX_CALIBRATION=retain-thread`; click-sweep and visual-lint per frozen brief. Cross-family K3 review, GitHub CI, full foundation, provider SANDBOX/LIVE and production: NOT_RUN for this checkpoint.
- Before every commit: fresh fetch; own `origin/unit/lc-u2b2-customer-link` absent at observed checks, so no remote merge is falsely claimed. Work is commit-only; no push, PR, force-push, deployment, real message or money action.
- All owned local processes finish before handoff; shared PG remains runner-managed. Unit is held for runtime acceptance and a permitted commit, not marked complete.

## Approved fixture extension — current uncommitted work

Parent owns the harness/Graph adapter, suite registration and browser spec; test_worker owns the new `tests/foundation/browser_inbox_customer_link_fixture_test.go` (same worktree, disjoint write paths). Independent read-only privacy reviewer checked CAS/auth/calibration isolation. Exact deployed model identifiers unavailable. No recursive delegation or product backend edits.

- Qualified actor signs in through actual MOCK OIDC; the original live_operator still supplies the denied-customer-read matrix. Test-only bounded actor selection requires a random header credential; it does not fabricate sessions.
- Two signed comment intakes and successful real private-reply Finish commands establish the shared peer link. Actual A16 bank-transfer orders are confirmed/cancelled through production handlers. Customers/A13 are asserted under scoped runtime RLS before browser startup. No replica mode or fabricated order/projection rows. Existing inherited store currency is USD; A13 has no currency field and the frozen UI explicitly displays its supplied claim minor value as TWD.
- Six INU10 cases use two independently signed real pages: B links/reloads; stale A receives 409, keeps its choice and shows the winner; only an explicit click retries, then reload/unlink/reload. Claims B2 quantities 2+1, session, TWD21, confirmed/cancelled orders, ordinal1 and sent auto-reply have independent literal assertions.
- Existing exact two-send Graph assertion now measures the browser delta after the two fixture-only private replies. New spec is in the inbox registry, diagnostic filename allowlist and GATES.md. INU05 is unchanged and remains mandatory in the same mode.
- Privacy review P2 was folded in: request URLs are captured in memory across reloads and checked raw/decoded, without writing URL history to artifacts; the Go log scan includes the new synthetic customer names. Source-only review; runtime mutant validation NOT_RUN.

| Current command | Actual exit | Evidence / boundary |
|---|---:|---|
| Playwright `test --list` (inert collection-only env) | 0 | `fixture-spec-list.log`: 19 tests / 3 files, all six INU10 variants registered; no browser launched |
| `go test -tags browser ./tests/foundation -run '^$'` | 0 | `fixture-go-compile.log`; compile only, no DB assertions ran |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `fixture-admin-types.log` |
| Strict tsc on all three inbox specs | 0 | `fixture-spec-types.status.json` contains full args |
| `bash scripts/dev/check-gates.sh` | 0 | `fixture-gates.log`; registration/header checks |
| `bash scripts/dev/test-node.sh` | 1 | `fixture-node.log`: first batch 410 pass/3 failures. team-bff listen EPERM; two deploy-prep Docker socket failures. Script stops there; later batches NOT_RUN |
| Focused six inbox Node files | 1 | `fixture-focused-node.log`: 64 pass/1 failure; inbox-bff loopback listener EPERM |
| `git fetch origin` | 255 | `fixture-fetch.log`: `.git/worktrees/.../FETCH_HEAD` Operation not permitted |
| `docker info --format '{{.ServerVersion}}'` | 1 | `fixture-docker.log`: Docker socket permission denied |

The new normal browser/real-PG seed, product-red baseline, normal-green and exact-INU05 calibration are **NOT_RUN**, not PASS and not a valid red. The known denied loopback/Docker permissions prevent them, and fresh remote merge is itself blocked. No permission bypass, fake response, weakened threshold or substituted fixture was used. Old 9edb browser evidence is preserved and cannot validate this changed source.

CI gates after remote synchronization/commit in a capable environment: normal `bash scripts/dev/test-local.sh --browser-inbox`; then `LC_INBOX_CALIBRATION=retain-thread bash scripts/dev/test-local.sh --browser-inbox` serially, requiring exactly INU05 and the other 18 browser cases green. Also the frozen brief's click-sweep and visual-lint, full Node and K3 review remain pending. Do not push without the integrator batch review.

No new commit exists: last committed checkpoint is `3440ef6c96eb9b90908873daceff584f1ea7d2f1`. Fresh fetch/own-remote merge must succeed before `git add -A && git commit`; the runtime's read-only Git metadata forbids that step here. The uncommitted diff plus untracked files are preserved; current hashes/status live in main output `fixture-source-manifest.json`. This is E1 source delivery, not full unit completion.

## 2026-10-09 WIP status check

Decision: KEEP all seven tracked modifications and both untracked test files as a coherent WIP. The exact fixture requested in FIXTURE-REQUEST is implemented and wired; no additional scope approval or external fixture author is needed. Independent source audit found no definite missing stub that prevents a WIP checkpoint. This does not assert the seed or browser cases pass. The inherited USD fixture remains only UI TWD-format evidence, not a real TWD transaction fixture.

Current HEAD is still `3440ef6c`. All nine file hashes matched the previous source manifest before this status note. Fresh `git fetch origin` exited 255 (`FETCH_HEAD: Operation not permitted`), so mandatory own-remote/trunk synchronization and the WIP commit could not run. The current runtime still denies Docker socket access (exit1) and loopback listening (EPERM, exit1). Evidence: main output `status-20261009.json` and its three logs. Normal19 browser cases, real-PG seed, product-red/green and exactly-INU05 calibration remain NOT_RUN.

Preserve the two untracked files in any later checkpoint: `tests/admin/inbox-customer-ui.spec.ts` and `tests/foundation/browser_inbox_customer_link_fixture_test.go`. In a capable environment first fetch/merge own remote if present and merge `origin/r3/integration` (including Go/tooling and registry changes), resolve the inbox registry as a union, then commit the WIP. Do not substitute old13-case evidence for current acceptance. No push or other unit work was performed.
