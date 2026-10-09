<!-- Purpose: record the W5 real shared-helper error correction evidence.
Depends on: worker6d61a1d4 source and original raw evidence SHA256=540da2bd2938a0d05007d655c7cd6869e06dcd396dbd3b2d47553dc2112bfe7e.
Used by: root independent checks and integrator acceptance. -->
# W5 wire real error-envelope P1 follow-up
- task_id: a83bad3d-2b4d-4b4a-b598-85f9e4a99cea; own subcanvas codex-w5-u1-ui-sub-wire.
- Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/w5-u1-wire-worker; branch unit/w5-u1-wire-worker.
- Merge prerequisite: merged --no-edit unit/w5-u1-import-wizard-ui without conflict, resulting base 86f638b54ba54473c3cd9353fb203a4dbcfcceca. Origin PROCESS ba1d157 rule consumed. Other owners preserved.
- Source SHA: 6d61a1d49cad986ed89c70565330418cb8cca5b1 (059855c3 implementation + 6d61a1d4 actual DELETE-request test). Assigned ui_worker/gpt-6.1-sol/high; actual runtime identifier UNKNOWN.
- Changed source/test paths ONLY: apps/admin/lib/import-client.ts; tests/admin/import-client.test.mjs; tests/admin/import-wire-real-loader.mjs. No auth.ts/proxy/backend/schema/props/DTO changes.

## Root cause and fix
The previous test supplied a handwritten localError/auth stand-in with private,no-store. Actual shared auth.localError emits no-store. sendImport checked private-only before reading coded errors, replacing every legitimate refusal with retry_later and uncertain=true after commit.
Client now accepts no-store (never public) ONLY for a closed five-key error envelope: code/message/request_id/retryable/details; expected32hex request ID, empty details, correct retryable flag, known code with matched status. Includes401 unauthorized,403 forbidden,404 not_found,409 idempotency_conflict,422 file refusals and invalid_request at413/415/422. Unknown/malformed/extra/data-bearing/mismatched envelope stays retry_later/uncertain after commit. Success200 and fresh-preview409 still require exact private,no-store and frozen DTO projection. Preflight refusal dispatches nothing; lost/session-changed post-dispatch commit remains UNKNOWN; no automatic retry. Other transport/timeout/CSV behavior unchanged.

## Real seam tests / framework boundary
Relevant old tests were replaced with actual TS route, actual native Request, actual auth.ts/settings-client/models/proxy, actual localError/cookie/CSRF/store authorization functions. Only fetch network edges are faked: client HTTP edge invokes actual handler+Request and upstream edge returns synthetic responses. Browser-only DOM/cookies/object-URL facilities are represented in Node to drive actual csrfCookie/sessionBoundary and capture exposure. No handwritten business/shared helper stand-ins remain.
Loader reuses existing registerHistoryTestLoader (unchanged), resolves @/ aliases, and resolves server-only to the INSTALLED next/dist/compiled/server-only/empty.js (actual react-server marker, not a handwritten stub). No Next build/server/browser is invoked. Other history writer may reuse this helper; its files are untouched here.
Coverage: real no-store401/403/413/415/422/409 errors client-visible; preflight vs sent refusal; actualCSRF/Origin/session duplicate-cookie/no-store/method405 with real DELETE carrying a body; store404/permission403; raw original upload/trusted bearer/privacy200+409; no-private success/stale rejection; unknown/malformed/extra/details/retryable/status/cache error negatives; dispatched loss/session/cancel UNKNOWN; failed download closed projection and final cancel zeroURL/click.

## Actual commands / evidence
Commands/exits and source SHA256 are evidence.json. Pre-fix actual-source red1 (red.log); isolated reintroduction of cache bug red1 (red-real-envelope.log), restored in finally -> green0,20tests (green.log). Native admin tsc0 (typecheck.log), strictheaders0 (headers.log), git diff --check0. Original red includes two harness corrections subsequently fixed; isolated red-real-envelope is the clean production-gate regression evidence.
Evidence class MOCK network/Node actual seam; author check pending root independent source review and rerun. No release/full UI completion claim.

## NOT_RUN / remaining
Next build/browser/PG/Go/full foundation/production/real network NOT_RUN per task local-resource policy. Root owns final docs/GATES/runner and finalsource artifact regeneration. API/error/body statuses are unchanged. No processes/temporary fixture dirs left; no push/release merge. This worktree's preparatory parent-branch merge is local only. Own memory/store/canvas updated before handoff.
