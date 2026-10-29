# RVFIX #221 — RED candidate, 2026-10-29

Authority: integrator CARD RVFIX #221 and final findings in read-only main checkout `output/integrator/tools/logs/cxf-rv-221.log`; frozen `contracts/storefront-v2.md:400–405` R1–R6. Read AGENTS.md and 架构.md sections 0/1/2/26/27. P1 is the session-boundary mismatch deadlock in Design; P2 storage-write failure is deferred to a PR comment as explicitly dispatched. No production implementation is changed before assertion-level RED.

Assigned worktree: `/Volumes/data/live_commerce_architecture_v1/.claude/worktrees/oc11`; branch `unit/oc11-publish-pending-offline`; PR #221 into `r3/integration`. Role: dispatched implementation worker, Codex; runtime model/effort not exposed. Starting head `4bc41efa19c6f611ce74c5e406ff55b8f73270bf`; merged integration `f8fd8900577815a328643ed878d88a6a247d2ab2` first (merge `f06e7c72c`). No recursive delegation. Humaux/coord/code-index tools unavailable after tool discovery; no store, claim or canvas update is represented as performed. Existing dirty DELIVERY.md and untracked evidence are preserved and excluded from this commit.

## Acceptance gate and click ledger

One batched tests-only RED push adds all six locale × transport cases to the existing dedicated store-design suite. No existing assertions, waits, timeouts, fixtures, exemption lists or gate adapters change. CI must fail at its own `RVFIX-P1 a new login must offer a usable explicit recovery path` enabled assertion on the unchanged implementation; setup/compilation/transport failures do not count. Only after that receipt is read may implementation proceed. Current-head GREEN and independent review remain pending; historical GREEN on 4bc41efa1 is not acceptance of these new tests.

| Page/control | Real user action | Expected visible/persistent result | Actual |
| --- | --- | --- | --- |
| Design, each locale | Edit and Publish with lost committed/uncommitted answer | Pending; original request journal; Publish disabled | NOT_RUN, CI pending |
| Account → Sign out → identity-service Sign in | Click production logout/login in the same tab | New authenticated session; old identity retained; no automatic replay | NOT_RUN, CI pending |
| Design after return and reload | Read pending and click Confirm publish result | Usable recovery control and explicit localized warning, not a deadlock | NOT_RUN, CI pending |
| Recovery confirmation | Cancel | No POST, unchanged old journal, Publish remains locked | NOT_RUN, CI pending |
| Recovery confirmation | Accept | Original key/body POST receipt, exact committed answer where applicable; pending clears | NOT_RUN, CI pending |
| Design reload → edit → Save draft | Reload, fill, save | Cleared journal stays cleared; Publish is usable for a changed draft | NOT_RUN, CI pending |

`page.evaluate` only reads the journal; route interception only cuts transport. Login/session identity is never injected. A two-second observation window exceeds the complete existing automatic retry schedule to reject cross-session automatic replay. No direct API call substitutes for a product interaction.

## Three-language copy contract

Only the test oracle defines this proposed explicit recovery confirmation; product copy is not implemented in RED.

| zh-TW | zh-CN | en |
| --- | --- | --- |
| 確認先前草稿 v{n} 的發布請求？如果尚未完成，這會將該草稿發布給買家。 | 确认之前草稿 v{n} 的发布请求？如果尚未完成，这会将该草稿发布给买家。 | Confirm the earlier publish request for draft v{n}? If it has not completed, this will publish that draft for buyers. |

## BOURDIEU

Existing screen/visual decisions are retained; this is a regression test for session recovery. Q1–Q5: no new visual or audience verdict claimed in this tests-only stage. Novice/expert means recent task experience. Q6 aesthetics: UNKNOWN until owner; no aesthetic change proposed.

## PROJECT UNDERSTANDING

Architecture §0 requires execution evidence distinct from a claim; §1 keeps transaction authority in Go/PostgreSQL; §2 fixes store/business boundaries; §§26–27 require worktree isolation and independent acceptance. R1/R2 bind recovery to the original transactional receipt; R3 prohibits replacement keys and list-derived success; R4 defines terminal outcomes; R5 retains pending through reload. The review additionally requires a usable explicitly authorized recovery path across a fresh login, preserving the automatic session fence.

## MOBBIN → EAGLE

No design or reference import is performed in this tests-only stage. Existing Design layout and native confirmation are retained. No new pattern/reference is needed or claimed; existing screen references remain in the parent DELIVERY.md.

## FIGMA OFFICIAL GUIDANCE

No Figma node, frame, prototype, token or visual asset is changed. Figma guidance application and copy lint are not newly claimed; this stage only extends browser regression coverage of existing product controls. Desktop/mobile design parity checklist is not applicable to this tests-only diff.

## Scope, evidence and handoff

Authored changes: `tests/admin/oc/design-publish-pending.spec.ts`, `output/oc-11/rvfix/DELIVERY.md`, appended `output/oc-11/red.log`, and mandatory generated `output/ci-select-backend-browser/browser-ownership.json`. The spec and generated ownership are outside the original card write_paths and are listed in the PR body under this review-fix dispatch. No source imports change, so component harness updates and Go dependency-map regeneration are not applicable.

Local static `node --check tests/admin/oc/design-publish-pending.spec.ts` and `git diff --check` exited 0 (E1 syntax/whitespace only). The ownership generator is static analysis; its result is regenerated last before the RED commit/push. No local tests, servers, builds, browser, PG, containers, formatters or dependency installation. Author assessment E0; no completed fix or GREEN claimed.

After the RED push, exact SHA/Gates/Foundation run and job IDs are recorded in the PR comment and local `output/oc-11/red.log`, without an evidence-only push. A fresh integrator session relays the CI conclusion; no silent background process is left. Same blocker is limited to two targeted attempts. No merge, auto-merge, branch overwrite, status posting or production operation. NOT_RUN: current-head CI execution/RED/GREEN until received; product fix; independent review; local runtime verification; LIVE.
