# W5-U1 wire subworker delivery
- task_id a83bad3d-2b4d-4b4a-b598-85f9e4a99cea; subcanvas codex-w5-u1-ui-sub-wire.
- Worktree /Volumes/data/live_commerce_architecture_v1/.worktrees/w5-u1-wire-worker; branch unit/w5-u1-wire-worker.
- base_commit dc11b5ce8ceaf3c755d7192545f34f3c579108cf; implementation commit fc8c2c6de2b4a175f639cec2c3b461b87510df93. Assigned ui_worker / gpt-6.1-sol / high, actual runtime model UNKNOWN.
- Changed paths (11 source/test paths, exact list+SHA256 evidence.json): apps/admin/lib/import-{model,request,header,client,copy,bff,proxy}.ts; app/api/stores/[store]/imports/[param]/[action]/route.ts; tests/admin/import-wire-{model,header}.test.ts; tests/admin/import-client.test.mjs. No shared contracts/backend/lockfile/scripts or other owners touched.

## Frozen API
ImportKind customers|orders; ImportMapping Record<string,string>. SafeImportRow row/outcome/code?/consent_ignored?/warning? has NO external_id. ImportPreview exact backend counts/hash/headers/mapping with safe rows, customers consent_ignored_rows vs orders city_dropped_rows. Receipt exactly batch_id/created/updated/failed/replayed.
Client exports readImportHeader(Blob), guessImportMapping(kind,headers), importFields/requiredImportFields, sendImport({store,kind,action,file,mapping,expectedApplyRows?,boundary,signal}) -> preview/stale/receipt(value) or error(code,uncertain), downloadImportFailures({store,batch,boundary,signal}) -> done|signed-out|forbidden|not-found|unavailable|uncertain.
Copy contract as frozen with root: three locales, plain wizard keys, Record<string,string> stages/errors/outcomes/fieldLabels and string confirmLabel. Invalid-field reasons are specific; UNKNOWN instructs deliberate same-file/same-settings retry, no automatic retry; idempotency conflict uses plain mapping/file correction text. No mapping persistence is implemented.

## Implementation
- Header-only UTF-8/BOM/RFC4180 reader slices <=1KiB chunks until first header record, never decodes or displays data rows; original immutable Blob uploaded untouched, <=2MiB. Alias guesses are header metadata, ambiguous aliases remain manually unmapped.
- Four exact POST resources plus failed-only result GET share one imports/[param]/[action] leaf to avoid Next kind/batch parameter collisions. Query closed mapping <=2KiB, values <=100 characters, whole encoded query <=4096B; expected_apply_rows <=5000 on commits; no Idempotency-Key, no tenant/amount/env params. Actual orders count units, while row numbers identify first data line (<=5000), so row can exceed rows_total.
- Existing server session/store membership and customers:privacy, Origin/CSRF enforced. GET uses exact Origin or absent Origin + same-origin Fetch Metadata plus CSRF. API leaf provides native transport to policy proxy (existing architecture permits fetch only in API/client). Transport75s + incoming abort covers capped raw body and trusted-origin bearer-only HTTP; no caller Cookie/BFF key/Authorization, redirect:error/no-store, never retries.
- 200 previews and direct 409 fresh previews are closed validated/projected without every external_id; failed IDs must be empty. Counts match row outcomes/erased/consent/city flags. Coded409 idempotency_conflict stays coded; receipts validate five keys plus expected applicable count. Safe local coded errors never forward backend messages/details.
- Post-dispatch lost/unreadable/session-cancelled commit remains uncertain; preflight no dispatch is definite. Browser90s covers store lookup plus75s. No async gap after final synchronous abort/hidden/cookie download fence.
- Backend results.csv?only=failed is UTF-8 BOM row,external_id,outcome,code; validate failed-only rows/closed codes/empty source IDs, emit BOM row,outcome,code only. Actual Go writeAttachment sets no-store (overwrites private), accepted only for successful attachment with exact filename/type; BFF normalizes private,no-store/nosniff. JSON requires private and no-store. No raw cells or IDs ever reach download.

## Actual commands / evidence
Command strings and exit codes in evidence.json. Red1 actual unsafe projector (red.log), mutationred1 privacy bypass (mutation-red.log) -> restoredgreen0,18 actual-source Node tests (green.log). Native admin tsc0, strictheaders0, UIarchitecture0; git diff --check0. check-gates1 ONLY new three Node suites unregistered by shared runner; root owns registration, no standards changed. Local source hashes bind evidence to implementation commit.
Evidence class MOCK transport / pure actual function automation, author self-check awaiting parent independent review. No browser/PG acceptance claim.

## NOT_RUN / unresolved
- Next build, browser/real click/storage flow, PG/Go/full foundation/production/real long-query wall-time NOT_RUN per local-resource policy.
- Root owns Wizard/navigation/CustomerDetail integration and shared test runner/GATES/browser gate. Register the three Node suites, independently re-run, and consume customer commit receipt only for the current scoped order-step readiness. There is no backend readiness endpoint.
- All-failed preview/commit writes no batch (nothing_to_apply); root generates local safe failure rows before commit; this worker downloads only an actual acknowledged batch.
- Alias coverage remains backend's published best-guess list; owner can manually map different headers. History writer separately owns import-history modules.
- No processes/temporary fixture folders left running; no push/merge. Original files never persisted to browser storage; no real buyer/credential fixture used. Humaux own canvas/store completed before final handoff.
