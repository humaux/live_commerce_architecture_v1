# FEAT-LIVE-REPLAY — prepared implementation, BLOCKED on assigned R2 pin

PR #396 remote head remains the tests-only b78ca35aaf41dfb978040edb1a148774f6f69049.
Local branch unit/feat-live-replay merged origin/r3/integration f59e5c540 before route changes.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/feat-live-replay.
Executor: Codex primary dispatched lane; runtime model and reasoning setting UNKNOWN (not exposed by this endpoint).
No child agents. Humaux/coord/code-index tools are unavailable; no shared-memory/coord writes claimed.

## Implemented locally (not pushed)

- Additive storefront-v2 / live-console-v1 amendments, preserving OC-Q9 and explicitly superseding
  its backend planned restriction; live-session-flow OpenAPI includes merchant and buyer replay shapes.
- Allocated migration 0213_live_replay.sql adds private-by-default independent publication CAS,
  durable official video references on future existing fenced snapshot completion, FORCE RLS,
  restricted reader definers and origin-only buyer projections. No old-session backfill or deletion.
- Existing Meta live_videos GET requests permalink_url; normalizer retains only a safe official
  Facebook video permalink. Existing snapshot/picker DTO accepts the optional field.
- Merchant GET/PUT enforce live:read/live:manage, ENDED, stored video, scoped key+principal receipt,
  version conflicts, audit and auth rechecks. Buyer list/detail reuse CURRENT catalog product read.
- Exact admin replay BFF reuses signed session/CSRF proxy; public storefront replay BFF reuses
  verified host origin and BFF key. No UI pages or customer messages.
- Extend route parity expectations for exactly the two new reads (46 legacy + 2), with independent
  exact grammar/method expectations. The frozen legacy oracle and gate-adapter assertions are untouched.

## Evidence

Causal RED E3 (failure proof only): Gates 41093988979 / job 124202749403 at tests-only head
b78ca35a, tested synthetic merge b009564a4522eef75a1da8ff09256f44974cbd93.
New tests actually compiled and failed their permalink request/normalization assertions; see red.log.
Foundation 41093988555 / 124194858726 passed static checks, not feature GREEN.
Current prepared-source hashes: source-sha256.txt (excludes this evidence document).

Static E1 commands/results on prepared source:
- go vet ./internal/live ./internal/httpapi ./internal/buyerhttp ./internal/integrations/metareply: 0.
- gofmt on changed Go files: 0; git diff --check: 0.
- node scripts/dev/derive-browser-ownership.mjs, then --check: 0 (after integration merge).
- bash scripts/dev/depmap.sh: 0, no dependency-map diff.
- python3 scripts/check_packet.py: 0; its incidental timestamp output restored to HEAD.
- pnpm --filter @live-commerce/admin exec tsc --noEmit --incremental false and storefront equivalent:
  NOT_RUN, tsc executable absent (pnpm reports ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL).
No local tests, development servers, PG, browser acceptance, containers or dependency installation.
Implementation CI GREEN, REAL_PG after implementation, BFF runtime, SANDBOX, LIVE,
UI PARITY-NEWFEAT, independent review, merge and deployment: NOT_RUN.

## Authority and retained evidence

Read AGENTS.md; architecture sections 0/1/2/26/27; storefront-v2, live-console-v1,
live-planning-v1 and owning code/migrations. Official Meta LiveVideo and Page live_videos pages
read with Kimi WebBridge on 2026-10-31 (Graph API v26.0). permalink_url exists; Page read requirements
use existing pages_read_engagement/pages_show_list, Page token/MODERATE and Live Video API feature.
No new scope. Embedded-video-player official guide also read. SOURCE only; no LIVE grant assertion.
Fleet Bridge read-only figma_status + get_design_context for 6:21646 and 6:21711, no Figma writes.
The browser task session was closed. Source docs/snapshots/raw CI log retained in this card directory.

## Blocking prerequisite and handoff

Card allocates migration 0213 but no replacement R2 pin. Current pin 139 is unchanged, per explicit
prohibition on choosing one. Integrator assignment requested. No implementation push is made while
this known gate prerequisite is missing; WAIT_RUN for implementation: NONE.
After assignment, apply only that pin, then push one implementation batch and record the new exact
Foundation/Gates workflow run IDs (never plan job). Poll at most 10 minutes, then relay.

Out-of-scope edits: none beyond explicitly authorized owning modules, Meta extension, contract/OpenAPI,
card evidence and route acceptance tests. No baseline/dispatcher registry/lockfile/go.mod changes.
The integration merge imports upstream shell work; this lane did not author that upstream diff.
No completion claim; own implementation assessment is E0, static readiness is E1.

## RELAY 2 — exact-head RED reverified, pin allocation still BLOCKED

Fresh PR API head remains b78ca35aaf41dfb978040edb1a148774f6f69049; local prepared
implementation is e937748714c1cbe93026ee98ffe428f334d8b044, not pushed. Raw GitHub jobs API
logs reverified unit job 124202749403 and g10 job 124202749598 in Gates 41093988979.
The g10 tests also compiled and failed their own assertions: durable live-video persistence
is absent and merchant replay GET returns 404 instead of 200. Both receipts test synthetic
merge b009564a4522eef75a1da8ff09256f44974cbd93. E3 failure evidence only, no feature PASS.

Fresh fetch still resolves integration to f59e5c5402a26021ca498d8323bc0e7ee8693c36 with
R2 pin 139. Current human card allocates 0213 but gives no replacement R2 pin, and explicitly
prohibits selecting one. Targeted Humaux lookup found the prior unresolved allocation record;
no replacement allocation was found. STOP without implementation push or pin edit. Integrator
must assign the exact pin before the implementation batch. No repair attempt spent this relay.

Humaux tools became available during this relay; earlier unavailable-tool statements describe
the prior preparation session. This relay records its own child canvas and problem under a lock;
it does not claim another task or write the integrator's canvas element. No new tests, servers,
builds, browser, PG, installation, generators, production changes or independent acceptance ran.
Evidence logs retained in this card directory; no task process remains. No new source paths.
WAIT_RUN 41093988979 is the existing tests-only Gates workflow; g04/g06 were still running.
Integrator relays its completion/failure/cancellation and supplies the pin; implementation CI
and GREEN remain NOT_RUN. No evidence-only commit/push was made.

## RELAY 3 — allocation still missing

Re-read PR396 head and both completed raw RED jobs through GitHub API: remote head
b78ca35aaf41dfb978040edb1a148774f6f69049, run41093988979, unit124202749403 and
g10124202749598. New assertions failed as documented above (E3 failure evidence).
Local implementation remains e937748714c1cbe93026ee98ffe428f334d8b044, unpushed.
Live remote integration remains f59e5c5402a26021ca498d8323bc0e7ee8693c36; exact
R2 count assertion remains139. PR comments and targeted Humaux lookup provide no
replacement allocation. The card still prohibits selecting one. Async question
requests the exact pin; only that allocation can unblock the implementation batch.
No source/test/pin edits, new tests/servers, generators, commit or push this relay.
No new outside-card source paths. WAIT_RUN implementation: NONE; GREEN NOT_RUN.
Only card evidence files appended. Humaux tools became available after discovery;
initial transport failure on canvas get recovered on retry. No recursive delegation.

## RELAY 4 — causal RED reverified; assigned R2 pin still missing

On 2026-10-31, fresh PR396 API read confirms remote head b78ca35aaf41dfb978040edb1a148774f6f69049.
Fresh fetch resolves origin/r3/integration to f59e5c5402a26021ca498d8323bc0e7ee8693c36;
its R2 assertion at tests/foundation/r2_integration_upgrade_test.go:132 still pins 139.
The relay/card and PR issue/review comments supply no exact replacement allocation.
The card explicitly forbids selecting a pin; no source/pin edit or implementation push.
Local prepared implementation remains e937748714c1cbe93026ee98ffe428f334d8b044.

Fresh raw GitHub job logs confirm causal RED (E3 failure evidence only): Gates
41093988979, unit job 124202749403 (missing permalink request/normalization) and g10
job 124202749598 (missing durable video row; replay GET 404 instead of 200), tested
merge b009564a4522eef75a1da8ff09256f44974cbd93. gh run view --log-failed refuses
until workflow completion; the direct jobs API logs succeeded. Workflow API currently
reports queued/conclusion null; Foundation 41093988555 is completed/success on RED head,
not implementation acceptance. No new workflow was dispatched.

Required clarification sent to integrator: assign the exact R2 migration-set pin for 0213.
Humaux/coord tools are unavailable in this relay; no store, lock or canvas update claimed.
No local tests, servers, builds, generators or new static checks ran. No new outside-card
source paths. No child agents or task processes left running. Evidence-only local append;
no evidence-only commit/push. GREEN/implementation CI/independent review remain NOT_RUN.
WAIT_RUN: 41093988979 (existing tests-only Gates run, integrator completion relay).
WAIT_RUN implementation: NONE (BLOCKED on assigned pin).

### FIX1 relay6 (2026-10-30T18:14:56.590052+00:00)

Fresh base Gates41098492538 is success at f59e5c540; detailed current failure table/readback is in FIX1.md. Integration merge and browser ownership regeneration/check exit0, unchanged. PR remote b78ca35aa remains tests-only; local e93774871 unpushed. Exact R2 pin still absent from both original cards; explicit allocation requested. BLOCKED before implementation push; E3 historical RED only. No local tests/servers, source/pin changes, commit, push or deployment. WAIT_RUN: NONE.


## FIX2 integrator-granted repair (2026-10-31)

Current disposition is FIX2.md first section; earlier BLOCKED entries are historical. Buyer dispatcher candidate + exact +2 route oracle are updated under the explicit grant; replay BFF now uses recognized static const helpers. No permalink assertions changed; HTTPS Facebook trust boundary stays. Full failure table and original CI pointers: FIX2.md. Closure candidate for independent PR review: output/buyer-route-table/FEAT-LIVE-REPLAY-FIX2-pin-candidate.md. Static E1 only; PR CI and E4 PENDING. No local behavioral tests/servers, no UI work or production action. Humaux unavailable this session.
