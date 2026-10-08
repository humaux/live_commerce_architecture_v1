<!-- Purpose: hand off the LC-U2b2 UI/BFF checkpoint with measured gates and the remaining positive-browser fixture gap.
Depends on: base abe4714f, frozen live-console-v1 A9/A13/A14 and LC-B3b, current Node/browser receipts.
Used by: integrator scope ruling, K3 review and later complete acceptance; not a declaration that all unit gates passed. -->
# LC-U2b2 customer link / BuyerPanel checkpoint

**Full unit acceptance remains BLOCKED** by the no-Go test-fixture boundary in [FIXTURE-REQUEST.md](FIXTURE-REQUEST.md). The pending scope question is required before altering the Go browser fixture. No Go, SQL, migration or OpenAPI file was changed.

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

- REQUIRED and BLOCKED: real signed qualified actor + full A13 dataset, then link→reload, real two-page CAS→explicit retry, unlink, complete A13 rendering across all six variants. See FIXTURE-REQUEST; the async scope question remains unanswered. No fake successful browser response or skipped positive case was introduced.
- CI: rerun `--browser-inbox` normal + `LC_INBOX_CALIBRATION=retain-thread`; click-sweep and visual-lint per frozen brief. Cross-family K3 review, GitHub CI, full foundation, provider SANDBOX/LIVE and production: NOT_RUN for this checkpoint.
- Before every commit: fresh fetch; own `origin/unit/lc-u2b2-customer-link` absent at observed checks, so no remote merge is falsely claimed. Work is commit-only; no push, PR, force-push, deployment, real message or money action.
- All owned local processes finish before handoff; shared PG remains runner-managed. Unit is held for fixture scope resolution, not marked complete.
