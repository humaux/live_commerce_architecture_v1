# META-CONSOLE-V3 — session stream and merchant console

Status: DESIGN / NOT_RUN (META-00, 2026-10-28).
Purpose: adopt the approved three-column console and mobile tabs on the frozen stream DTO.
Depends on: META-INBOX-V2 and META-STREAM-READ accepted; shared BuyerPanel and design-state contracts frozen.
Used by: live operators and subsequent LC-U3-ATTEMPT entry wiring.
Invariants: exact recipient/source, existing message authority, accepting ≠ lifecycle, verified playback only.
Authority: `contracts/live-console-v1.md` §§2–7.0.1/9/11, intake §3 and META-00 R1–R4.

## Scope and write paths

- Left: official-player eligibility/fallback and products. Middle: ordered comments plus this session's eligible
  DMs. Right: selected buyer/product/reply summary. Mobile tabs preserve the same facts and safe recipient.
- All/keyword/private/unreplied consume the frozen read surface; non-bundle DM selection uses conversation ID.
  Reuse A4/A5/A12, no auto channel fallback, no consent branch, no edit-updates-quantity hint.
- 接單中/暫停接單 uses the existing StudioClaims accepting/window command and current CAS/permission checks;
  never start/end/archive a session as a pause side effect. Player UNKNOWN/ineligible/unavailable → plain state
  plus validated native Facebook watch link; no fabricated player or unverified official embed.
- Write paths: `apps/admin/components/{LiveConsole,CommentStream,CommentReply}.tsx`,
  `apps/admin/src/features/live/**`, `apps/admin/lib/inbox-bff.ts` (new read grammar only),
  `tests/admin/live-console*.test.ts`, `tests/admin/live-console*.spec.ts`, `output/meta-console-v3/**`.
  BuyerPanel is consumed unchanged from Inbox; any needed prop change returns to integrator for path assignment.
- No backend, schema, sending grammar or Studio lifecycle rewrite. Three locales zh-TW/en/ja; existing tokens
  and shared components. Do not touch unmerged DS/transport lanes. One worktree/branch/PR, no merge.

## Batch RED list (GitHub CI only)

1. 全部 contains comments + eligible DM; unrelated DM excluded; non-bundle DM opens the correct recipient.
   Switching rows with stale requests cannot expose the previous buyer/thread; all four filters/cursors work.
2. Pause changes accepting only, refresh persists it and lifecycle stays live; reopening uses the same switch.
3. Unverified/ineligible embed produces plain fallback/native link, never playback; verified official eligibility
   enables only the permitted player. Missing source has an honest no-link state.
4. Public reply refuses a pay link/PII, private quota/window/UNKNOWN controls stay effective. Comment edit/delete
   changes display only, no claim count or misleading quantity copy. No marketing-consent state.
5. Hidden tab pauses reads/clears private data; stale/cross-store responses cannot repaint. Mobile/review parity
   and enabled/disabled/loading/error/empty states retain the exact selected recipient and reply channel.

One batched tests-only RED then implementation push; old gate-adapter assertions remain verbatim and ordered.

## Acceptance gate and handoff

PR-own `--browser-live-console`, `--browser-inbox`, affected studio/live-claims regression, click sweep,
visual lint, Node/typecheck; preserve cadence, privacy and UNKNOWN assertions. Every actionable control gets a
real click/fill/keyboard ledger with visible outcome and refresh/PG persistence where applicable.
**Mobile/summary parity gate:** recipient, channel, product/qty, assignee, window and send result must appear
on desktop, mobile and every summary/review state; table in DELIVERY.md, missing = P1. Include 1440/390 all-state
three-locale evidence, exact SHA/run/job IDs and independent privacy/visual review. Separate Figma design edits
must supply parity/copy-lint receipts before design handoff. Seller copy excludes raw codes and review scaffolding.

NOT_RUN: implementation, CI, clicks, visual review and actual player eligibility until independently evidenced.
MOCK only; Meta SANDBOX/LIVE, real playback/DM, App Review, real-device probes and production are NOT_RUN.
