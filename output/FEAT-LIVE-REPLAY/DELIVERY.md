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
