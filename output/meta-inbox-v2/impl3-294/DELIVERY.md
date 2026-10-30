# IMPL3 / PR #294 — exact outbound correlation

Date: 2026-10-30. Assigned UI worker in `.worktrees/inbox-v2-ui`, branch `unit/inbox-v2-ui`, base `10ec3a0f50520b91b9c5423cf85b1096f8865f4a`. Runtime: Codex / GPT-6; exact model variant and effort identifier unavailable. No delegation or duplicate claim. Humaux tools are unavailable: search/rejected enumeration/store/canvas/index/link NOT_RUN; this delivery and the PR comment are the available handoff.

## Authority and implementation

Read AGENTS.md; architecture §§0/1/2/10/26/27; `docs/delivery/units/meta-inbox-v2.md`; original `contracts/live-console-v1.md` §11 A9/A12; current InboxThread/DTO, direct-consumer fixtures and exact-operation parser. Authority is the owner's IMPL3 instruction and integrator seam commits `2947549dc` + `10ec3a0f5`, not older output reports. A9 optional outbound_id now matches the exact A12 ACK outbound_id. No Go/contract/migration/seam edits.

Merged origin/r3/integration `21eaceb5c72ce6b55955607529daef31fe3163b7` with a merge commit, preserving both inbox and editor exports. Resolved only the i18n header conflict and regenerated browser ownership. Pre-existing dirty/untracked handoffs were preserved and excluded from staging.

The ACK retains operation ID, outbound ID and original submitted text. Validated exact-operation completion supersedes only the matching outbound projection. Every other queued/blocked projection is captured even during pending ACK reads and survives omission until its own explicit terminal projection. Matching uses outbound ID only; missing IDs retain a conservative lock. UNKNOWN remains independently latched. A rejected ACK's original text survives other obligations and is editable when they settle. Fixtures only gain outbound IDs; unrelated rows have distinct IDs. Assertions, strings, times and states stay unchanged. No imports, UI copy, settings, routes or layout changes.

## Acceptance and evidence

Existing causal RED re-read from GitHub APIs and failed-job logs: tests-only head `5d66cd1b3d9da52a92f16384608b3d320d6f869a`; Foundation run `40943430245`, job `123692452595`, 24 new local-ACK behavioral failures. Gates run `40943430541`, bundle:06 job `123694392124`, Inbox 12 failures / 40 passes per integrator receipt; browser per-assertion artifact was not independently downloaded in this dispatch. See red.log for raw filtered Foundation assertions. E3 applies to historical causal RED, not implementation acceptance.

Acceptance gate: current-head PR Foundation, Required-Gates, inbox native-click/persistence cases in all three locales, and integrator independent acceptance. No local tests, servers or browsers. Static checks and mandatory ownership generation are recorded in static.log. tsc NOT_RUN: no installed local compiler/dependencies; no install attempted. Implementation GREEN and independent acceptance are PENDING, author claim E0. Final head and native WAIT_RUN IDs belong in the PR comment; no evidence-only follow-up commit.

## BOURDIEU

Q1 novice/expert means recent experience replying/checking delivery, never class or device. Q2 retain buyer selection → reply → delivery check. Q3 one staff send cannot settle another. Q4 server IDs and exact-operation reads establish delivery authority. Q5 retain the same safety behavior and seller language in zh-TW, zh-CN and en. Q6 aesthetics remain UNKNOWN until owner; this card changes no aesthetic decision.

## PROJECT UNDERSTANDING

Architecture §§0/1/2 separate MOCK/LIVE and keep server facts authoritative; §10 requires message eligibility and takeover generation; §§26/27 require isolated ownership and independent acceptance. A9/A12 impose exact outbound identity, fixed recipient and delivery readback. The owner's IMPL3 ruling permits optional identity for older servers while requiring missing-identity rows to fail closed. No new business setting, permission or API authority is introduced.

## MOBBIN → EAGLE

Behavior-only correction using the existing signed inbox design. No new reference, visual pattern or asset is used; no Mobbin/Eagle access or import is claimed. Needed Mobbin query: none.

## FIGMA OFFICIAL GUIDANCE

No design export or Figma modification. Auto layout/components/variables/prototyping remain unchanged; no fresh guide access, copy-lint result or design acceptance is claimed. Existing signed design provenance remains in `output/inbox-v2-impl/DELIVERY.md`.

## Scope, rollback and handoff

Explicit IMPL3 extra path: `apps/admin/lib/inbox-types.ts` (optional A9 field and invariant header). Merge-conflict header only: `packages/i18n/src/index.ts`. Mandatory generated path: `output/ci-select-backend-browser/browser-ownership.json`. Report all in PR body/comment. Other integration changes came from the authorized merge. No Go dependency edge changed, so depmap is unchanged.

Rollback reference: `10ec3a0f50520b91b9c5423cf85b1096f8865f4a`; no destructive restore was performed. One implementation push only. No merge of the PR, auto-merge, status, production operation or deployment. Integrator relay is the CI wakeup; stop after registering workflow runs. No local background process, fixture, port or temporary cache was created. P2 for issue: existing timestamp display-order comparison, already tracked in PR comment 6489297640; unchanged.

NOT_RUN: local tests/server/browser/build/PG/container/LIVE, local tsc, implementation-head CI acceptance, native click/persistence/density acceptance, independent review, Humaux shared-state operations.
