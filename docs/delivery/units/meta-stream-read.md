# META-STREAM-READ — bounded session comment + DM projection

Status: DESIGN / NOT_RUN (META-00, 2026-10-28).
Purpose: implement the read-only session stream DTO; no new send or claim authority.
Depends on: accepted META-00 DTO in `contracts/live-console-v1.md` §7.0.1 and existing A2/A8/A9 readers.
Used by: META-CONSOLE-V3; Inbox remains the store-wide destination for unrelated DMs.
Invariants: exact source identity, explicit session membership, authenticated scope, bounded reads, no new plaintext store.

## Scope and write paths

- Implement `/live-sessions/{sid}/stream` exactly as §7.0.1: fields/nulls, all/keyword/private/unreplied filters,
  total ordering, vector cursor, late arrivals, epoch/linkage reset, per-source EOF and bounded scans.
- Include a non-bundle DM only with explicit session proof; a customer link alone or nickname is insufficient.
  Keep comment deletion/read eviction independent of accepted claim quantity (intake §3 / META-00 R1).
- Write paths: `internal/live/stream*.go`, `internal/httpapi/live_stream*.go`,
  `tests/foundation/live_console_*_test.go`, `output/meta-stream-read/**`.
  Go/SQL lane only; no UI/BFF/send rules. New SQL, root/post-River migrations, router composition or shared
  OpenAPI require integrator-assigned paths before edits. Migration numbers/R2 pin: **integrator assigns**.
- Preserve existing A2/A8/A9 grammar and readers. Missing exact membership proof stays excluded/BLOCKED,
  never filled with a guessed identity. Freeze the implemented DTO with integrator before UI starts.

## Batch RED list (GitHub CI only)

1. Same-session explicit non-bundle DM is included/selectable; unrelated same-store, same-name, other-session
   and customer-only-linked DMs are excluded. Cross-tenant/store/session and missing inbox permission disclose no DM.
2. Same-time/source collisions preserve distinct item keys; late old-event-time arrivals are not lost. Cursor
   tamper/scope/filter misuse is rejected; epoch or membership change clears the old view and resets safely.
3. Limits/continuations, raw-scan exhaustion and deletion coverage remain correct, including `seq:null`, empty
   filtered pages, incomplete scans and IG removed-final-comment rebuild; edits/removals never change claim quantity.
4. Per-source bounded scans/ring/TTL/demand and hidden-client pause are retained. Unreadable DM stays explicit;
   no private text/name/PSID/contact/token in new storage, logs, metrics, cursor or audit.
5. Read has zero read-mark/takeover/send/claim side effects and retains existing UNKNOWN/window/quota authority.

One tests-only RED push for all cases, then one implementation push. Preserve all previous assertions and pins.

## Acceptance gate and handoff

PR-own CI REAL_PG + fake Graph: existing `--live-console` / `--inbox` modes plus new projection assertions,
contract-shape parity, current exact grants/ACL negatives, Node/contract drift if the integrator wires transport.
Use actual migrated roles and primary readbacks; report selectors/skips, source/head/merge SHA and run/job IDs.
Independently review membership/privacy and hand the frozen DTO to META-CONSOLE-V3. A private-send success test
is not proof of session membership or real Meta delivery. No new UI is part of this card.

NOT_RUN: implementation and every CI/REAL_PG assertion until receipts; Meta SANDBOX/LIVE, App Review, real
messages, browser UI acceptance, production. Test provider is MOCK only; no credentials or connections requested.
