# FEAT-CONTACT-STORE — RED preparation

Evidence: **E1 static only**. No local tests or servers were run. No implementation or migration has been added yet.

## Authority and assignment

- Dispatched implementation card FEAT-CONTACT-STORE; branch `unit/feat-contact-store`, base `3ea359f796d36153bb4b8aa7259e4e0c27676b86`.
- Read `AGENTS.md`, `架构.md` §§0, 1, 2, 26, 27; `contracts/storefront-v2.md`, `contracts/meta-inbox-v1.md`; current buyer HTTP, merchant HTTP, PII/keyring and notify worker seams.
- Runtime: Codex implementer, model/effort not exposed as a verifiable runtime value. No sub-agents spawned. Main checkout read-only.
- Existing task delegated by integrator; no additional coord task claimed. Own canvas element: `integrator-sub-feat-contact-store`.
- Humaux: card provenance `772aa67a-5476-4226-a74c-8416e187aff9`; development rules `7bdb220b-0798-4b1d-aff5-16ee94cae75f`; rejected enumeration coverage 9/9. These are pointers; the current user card and repository sources govern implementation.

## Production Figma read (2026-10-30, Fleet Bridge, read-only)

File `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`. Read `26:850`, `26:1117`, `29:641`, `29:670` with `get_design_context`.
The visible contact page currently contains optional order number and message. The card explicitly also requires name and email; the tests retain both requirements for the subsequent UI card.
Success-frame copy currently says the buyer can view a reply. This does not authorize implementing outbound buyer email; merchant replies remain external as specified by the card.

Merchant message v2 root `1429:39076`, layout `1429:39077`, conversation `I1429:39077;1429:37280`: current readback shows Instagram conversations, shared read status, takeover and an Instagram reply target. It does not show website contact submissions. This card therefore specifies a separate minimal merchant contact list/detail/handled API; **UI: pending PARITY-NEWFEAT**. No modification of the Meta inbox list is planned.

## Planned endpoint contract under test

| Method | Go endpoint | Permission / scope | Result |
|---|---|---|---|
| POST | `/v1/buyer/contact-messages` | BFF secret + verified published origin; guest allowed, buyer bearer optional | 201 `{message_id,status:"received"}` |
| GET | `/v1/admin/stores/{store_id}/contact-messages` | merchant session, `inbox:read`, server-resolved store grant | 200 `{items,next_cursor}` |
| GET | `/v1/admin/stores/{store_id}/contact-messages/{id}` | merchant session, `inbox:read` | 200 detail |
| POST | `/v1/admin/stores/{store_id}/contact-messages/{id}/handled` | merchant session, `inbox:reply` | versioned detail; stale version 409 |

Public BFF: `POST /api/buyer/contact-messages`, via the existing catch-all handler. Body: `{name,email,order_number?,message,website?}`. Public response contains no buyer PII and never sets a buyer cookie.

Proposed bounded defaults under test: name 1–120 Unicode characters, email <=254 bytes, order number <=64 characters, message 1–5000 Unicode characters; whitespace-only required fields refused; plain text, no NUL/control injection; honeypot `website` empty or absent. Five new messages per IP and 100 per store per 600 seconds, with bounded `Retry-After`; accepted idempotent replay remains available after the limit. These new API details must be captured in an additive `storefront-v2.md` amendment with the implementation.

The implementation must define retention and its execution path in that amendment; no unsupported legal-retention claim is made by this RED package. Migration number reserved exactly: **0211_store_contact_messages.sql**. R2 pin remains unchanged in this tests-only stage; obtain the integrator-assigned value before changing it.

## RED suite and CI discovery

- `tests/foundation/store_contact_messages_test.go`: public admission, optional buyer session, durable retry, changed-body conflict, merchant list/detail, handling and version races, validation, synthetic scope injection, merchant permission/tenant/store refusal, unpublished origin, throttles, concurrent retries, server-derived stored scope, encrypted envelope, direct runtime table-read denial, existing notify worker + MOCK mailer, no inline sends, no duplicate sends, UNKNOWN never blindly retried and no PII in logs.
- `apps/storefront/tests/contact-messages.test.mjs`: real BFF handler with MOCK upstream; guest admission, strict body/JSON, bounds, honeypot, origin/key/method/content-type fences, exact upstream authority, strict receipt projection, no cookies, throttling, conflict and uncertain transport without retry.
- Go tests use only pre-existing production symbols and test-owned wire DTOs; the expected RED is the missing endpoint's HTTP assertion, not a compile failure.
- Existing CI discovery includes the Go foundation catch-all and `scripts/dev/test-node.sh`'s storefront test glob. No CI workflow or gate-adapter assertions changed.
- Generated `output/ci-select-backend-browser/browser-ownership.json` is refreshed with `derive-browser-ownership.mjs`, then `--check`. No browser spec/page/route was added or renamed in this RED stage.

## Static checks and handoff

`gofmt`, `node --check` and `go vet ./tests/foundation` are the only local code checks used. The browser-ownership generator/check and depmap generation are explicitly requested static generation. `go vet` exited 0; `node --check` exited 0; browser ownership `--check` exited 0. Depmap regeneration produced no change.

Push exactly one tests-only RED commit and open one PR into `r3/integration`; record actual Gates/Foundation workflow run IDs in `red.log`. Wait at most about ten minutes, then return `WAIT_RUN` for integrator relay. Do not push implementation before the new tests have failed on their assertions. No merge, auto-merge, force-push, commit status, live mail or deployment.

NOT_RUN: all local tests; local servers; CI RED/GREEN until receipts exist; browser/UI clicks (backend-only card); provider SANDBOX/LIVE; production; independent review. Contract amendment, migration, Go/BFF implementation, schema/OpenAPI and retention are pending the RED receipt.

Outside explicit product write paths: generated browser-ownership artifact and this required card evidence directory. No other module implementation changed.
