# FEAT-CONTACT-STORE — FIX1 implementation, current CI pending

Current: FIX1 implementation is prepared on PR [#375](https://github.com/humaux/live_commerce_architecture_v1/pull/375), after merging integration a3dc78e4d and refreshing to 629f42152 before push. The integrator's 2026-10-31 ruling explicitly authorizes migration 0211, contract amendment, contact handler/SQL, required BFF routes and one notify kind; R2 = 139+1=140. Prior STOP/relay sections below are historical. See [FIX1.md](FIX1.md) for every failure/class/action and the current handoff. **E3 historical causal contact BFF/Go RED only; E1 static structure; current-head GREEN and independent review PENDING. Author assessment E0.** No local tests/servers or migrations ran. No UI acceptance or production claim.

## FIX1 implementation and static evidence — 2026-10-31

- Added encrypted contact persistence and scoped read/CAS, public admission and private merchant BFFs. Preserve original RED assertions. No dashboard/page-path changes; click-sweep `todo-stock` is class (c)-candidate per ruling, for integrator rerun.
- No new transport or queue: one `store_contact` outbox kind, store-profile contact_email recipient, no buyer-mail or inline SMTP. Notice copy is zh-TW/zh-CN/en; content is generic, with no buyer PII. Existing notify worker also performs bounded 180-day contact retention, including dormant stores.
- `go build ./internal/storefront ./internal/buyerhttp ./internal/httpapi ./internal/notify`: exit 0. `go vet ./internal/storefront ./internal/buyerhttp ./internal/httpapi ./internal/notify ./tests/foundation`: exit 0 after tenant/store binding update. `gofmt` on changed Go source/tests: exit 0. Changed TS sources and admin test files `node --check`: exit 0. `python3 scripts/check_packet.py`: exit 0, PASS_PACKET_STRUCTURE_ONLY; its incidental packet-check result was restored because it is outside this card. These are E1 static receipts, never behavioral PASS.
- `node scripts/dev/derive-browser-ownership.mjs`: exit 0 (54 modes); `--check`: exit 0. `bash scripts/dev/depmap.sh`: exit 0. Both generated artifacts are included as required, not hand-edited.
- Local tsc unavailable (no compiler/node_modules executable); full tsc is NOT_RUN locally and belongs to CI. Added CI cases cover envelope tampering/old-key rotation, strict admin BFF projections/permissions and three-locale notify rendering. No new browser spec or product page.
- Humaux MCP/coord/code-index tools unavailable; equivalent trail is this delivery, FIX1 table, RED excerpts and PR body/comment. No shared-memory/canvas completion is claimed. No recursive delegation. CI run IDs will be reported in the PR comment/final relay; integrator is the agreed wakeup owner. One implementation push, no merge/auto-merge/evidence-only follow-up push.

NOT_RUN: all local behavioral tests/servers; current-head GREEN until CI completion; browser/UI clicks; independent acceptance; SANDBOX/LIVE providers; production deployment. Known limit: retention is bounded worker work and is delayed when workers are stopped; the 180-day policy is not an exact-time deletion guarantee or legal claim. API UI remains PARITY-NEWFEAT.
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

NOT_RUN: all local tests; local servers; feature GREEN and Go/PG RED; browser/UI clicks (backend-only card); provider SANDBOX/LIVE; production; independent review. Contract amendment, migration, Go/BFF implementation, schema/OpenAPI and retention are pending the RED receipt.

Outside explicit product write paths: generated browser-ownership artifact and this required card evidence directory. No other module implementation changed.

## Relay 1 — current-head receipt and hard-boundary STOP

Foundation `41056798375` / job `124066257869`, PR head `ca5a55e5926391aeb1a8eb8c55832e706a3f9d53`, failed in Node unit suites (storefront, i18n). All six new contact BFF tests reached their assertions (HTTP 404 versus expected 201/400/405/503/429). First failure: `contact-messages.test.mjs:46`, `public contact endpoint must exist before buyer login`, `404 !== 201`. **E3: causal BFF RED only**; the initial queued summary above is historical. Go/PG RED and feature GREEN are not claimed. Raw excerpt is appended to `red.log`. Gates `41056798358` remains pending on the relay read.

Implementation is stopped by the card's explicit boundaries:

- R2 pin: `tests/foundation/r2_integration_upgrade_test.go:131–133` pins 139. Neither the dispatched card nor the current integrator card/common originals assigns the new numeric target. The preamble forbids choosing it. Integrator must assign the target.
- Notify ownership: `contracts/storefront-v2.md` §E explicitly owns `internal/notify` and schema `notify`. `internal/notify/render.go:18–24` has no contact kind; `Render` at lines 321–324 rejects unknown kinds. The existing worker sends only renderer output. Required notification therefore needs an additive renderer/worker integration in that separately owned backend package (exact changes to be reviewed after scope handoff). The preamble explicitly requires STOP for unowned backend packages. No fake order notification, new queue, inline mail or weakened test is substituted.

Integrator unblock: assign the R2 pin and grant the minimal `internal/notify` integration ownership (or provide its frozen interface). No production source, contract, migration, pin or tests changed this relay; no new commit or push. Existing evidence files are kept local for the eventual single implementation batch. No local tests/servers were run. Feature incomplete; author PASS remains E0. UI: pending PARITY-NEWFEAT.

## Relay 2 — unchanged hard-boundary STOP

Re-read PR375 and exact-head GitHub receipts: HEAD `ca5a55e5926391aeb1a8eb8c55832e706a3f9d53`; Foundation `41056798375` / job `124066257869` completed FAILURE on the six new BFF HTTP assertions, not compilation. Gates `41056798358` remains QUEUED on this read. E3 proves causal BFF RED only; feature incomplete.

Current user RELAY 2 repeats the CI result without assigning the numeric R2 pin or handing off `internal/notify`. Re-read current source: `tests/foundation/r2_integration_upgrade_test.go:131-133` pins 139; `contracts/storefront-v2.md` section E owns the notify package; `internal/notify/render.go` supports no contact kind and rejects unknown kinds; `internal/notify/worker.go:169-176` sends only Render output. The explicit card STOP boundary therefore remains. No production/source/test edits, local tests/server, commit or push in Relay 2. Integrator next: assign exact R2 value and minimal notify integration ownership/interface, then relay. WAIT_RUN `41056798358` is the existing Gates workflow, not a plan job; integrator retains wakeup ownership. No new run was started.

## Relay 3 — escalation requires a pin/scope handoff

On 2026-10-30, `gh pr checks 375`, `gh pr view 375 --json headRefOid,baseRefName,body,comments`, and `gh run view --log-failed --job 124066257869` revalidated the same head and six new BFF assertion failures. The PR has no comments assigning either missing authority. Foundation run `41056798375` is completed/failure; Gates run `41056798358` is queued at the same head. No GREEN or Go/PG RED is established.

Current RELAY 3 again supplies no numeric R2 target and no ownership handoff for `internal/notify`. Read the required architecture sections and the original contract section E; rechecked pin 139 and the renderer/worker path in current source. The hard boundaries remain: do not choose the pin and stop before editing an unowned backend package. Escalate to the integrator for these two concrete assignments rather than relaying this same Foundation result again. No code, migration, test or gate changes; no local tests/servers, commit, push, merge or new CI run. Only this delivery record and `red.log` were appended locally. Humaux tools are unavailable in this runtime, so no Humaux store/canvas update is claimed. Existing Gates completion relay remains with the integrator; no background task was started here.

## Relay 4 — exact-head RED revalidated; assignment still required

On 2026-10-30, re-read `AGENTS.md`, `架构.md` §§0/1/2/26/27, contract §E, the R2 pin and notify renderer/worker. `gh pr checks 375` and `gh run view 41056798375 --log-failed --job 124066257869` confirm the same six new BFF assertions fail with 404 versus 201/400/405/503/429 (506 tests, 500 pass, 6 fail). Source remains `ca5a55e5926391aeb1a8eb8c55832e706a3f9d53`; **E3 causal BFF RED only**. `gh pr view 375 --json comments,headRefOid` returns no comments and the same head. Gates `41056798358` remains pending: bundle:03 job `124070185543` is in progress; other gate jobs are queued. Build success is not feature acceptance.

RELAY 4 still supplies neither the assigned numeric R2 target nor `internal/notify` ownership/interface. Current pin is 139 (`tests/foundation/r2_integration_upgrade_test.go:131-133`). Contract §E assigns notify to buyer-comms, and its renderer rejects an unknown contact kind. The card explicitly forbids choosing a pin or editing an unowned backend package; **STOP** remains mandatory. Integrator next action: supply both assignments, rather than relay this same Foundation failure again. No implementation, migration, tests or gate edits; no local tests/servers/checks, commit, push, new workflow or recursive delegation. Only local card evidence appended. Humaux search/store/canvas tools are unavailable; no shared-memory completion claimed. Existing Gates wakeup remains the integrator's relay. GREEN, Go/PG RED and independent review remain NOT_RUN; feature incomplete.
