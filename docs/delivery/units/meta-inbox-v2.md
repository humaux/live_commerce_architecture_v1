# META-INBOX-V2 — store-wide Inbox v2

Status: DESIGN / NOT_RUN; implementation brief, not a completion receipt (META-00, 2026-10-28).
Purpose: adopt Inbox v2 using the existing store-shared messaging authority.
Depends on: META-00, closure of overlapping PR #259, shared design-state contract, existing A8–A14.
Used by: META-CONSOLE-V3 and LC-U3-ATTEMPT through the shared BuyerPanel.
Invariants: authenticated store scope, fixed recipient, no inferred identity/consent, no automatic UNKNOWN resend.
Authority: `contracts/live-console-v1.md` §§3/4/6/7.0/11 and owner-choices OC-Q12/Q13.

## Scope and ownership

- Keep the store-wide Inbox; expose 全店共享 / assignee / store-shared read state, window cutoff and send result.
  Reuse A8–A14, including unread; do not redo the already-merged unread implementation or #259's repair.
- Adopt approved v2 desktop/mobile structure, list/thread navigation and fixed selected recipient. No marketing-
  consent badge/branch. Unknown delivery permits readback only; preserve A9's true/false/null delivery guard.
- Three locales zh-TW/zh-CN/en, 1440/390 layouts, existing tokens/components and accessible ≥44px touch targets.
  Seller copy uses plain language; no raw UNKNOWN/queued/MOCK codes or review scaffolding on product screens.
- Write paths: `apps/admin/components/{Inbox,InboxThread,BuyerPanel}.tsx`,
  `apps/admin/src/features/messages/**`, `tests/admin/inbox*.test.ts`, `tests/admin/inbox*.spec.ts`,
  `output/meta-inbox-v2/**`. No Go, new API, migration or shared DS/BFF edits; a missing seam is BLOCKED.
- Freeze BuyerPanel props before handoff; META-CONSOLE-V3/LC-U3 must serialize shared-file changes after this unit.
  Implementer uses its own integrator-assigned worktree/branch and one PR to `r3/integration`; no merge.

## Batch RED list (GitHub CI only)

1. Two staff see the same read state; read-only staff cannot clear it. Switching threads while an old response is
   pending never changes recipient or renders another thread's private text.
2. Window cutoff/capability refusal prevents send. UNKNOWN or null delivery authority cannot cause a second send;
   query/readback never manufactures success.
3. Hide/pagehide/logout/store change clears private text, cancels reads and prevents stale repaint. Sentinel text,
   names and sender IDs never reach storage, URL, console or BFF logs; cross-store access remains refused.
4. Mobile list/thread/summary retain recipient, channel, product/quantity, assignee, window and send result;
   absent product/assignee facts have explicit empty states rather than silently disappearing.

One tests-only RED push covers the entire list; assertions must fail on the missing behavior, then one
implementation push. Preserve old assertions, fixtures, budgets and delivery/window guards verbatim.

## Acceptance gate and handoff

PR-own CI: `--browser-inbox`, click sweep, visual lint, Node/typecheck and affected existing privacy regressions.
Every control uses real click/fill/keyboard; assert visible result and refresh persistence where applicable, plus
PG readback for shared state. API fixture setup and screenshots cannot substitute for user actions.
**Mobile/summary parity gate:** DELIVERY.md must map recipient, channel, product/qty, assignee, window and send
result across desktop, mobile and every summary/review state. Missing fact = P1. Add per-control click ledger,
1440/390 all-state evidence, locale coverage and current-SHA run/job IDs; independent privacy/visual review.
If Figma is changed by a separately authorized design unit, its copy-lint must pass and delivery must reference it.

NOT_RUN until exact-SHA receipts: all implementation/CI/browser/visual/privacy gates. Meta SANDBOX/LIVE, real
messages, App Review, production and real-device Meta browser checks are NOT_RUN; MOCK only, no activation.
