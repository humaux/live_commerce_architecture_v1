# RVFIX #221 relay1 — P1 implementation candidate, CI pending

This section supersedes the tests-only status below. Authority: this relay's card; final review P1 at Design.tsx:228; current source and frozen storefront-v2 R1–R6. Merged origin/r3/integration `10d6e8234b1b1f753606bf0b95abb593871da114` first (merge `862211dee`). Assigned implementation worker in oc11, branch unit/oc11-publish-pending-offline; Codex runtime model/effort not exposed. No recursive delegation or separate claim.

## Causal RED and repair

Tests-only head `fbaec2b35f5bd9504035c0ca81a41a9f07edf5df`: Gates run `40858325183`, bundle:04 job `123396762919`, artifact `12892706712`. Official playwright.log: **six new tests failed on their own enabled-control assertion**, all three locales × before/after-commit; 20 other tests passed. The failure is `RVFIX-P1 a new login must offer a usable explicit recovery path`, expected enabled / received disabled, spec line 123. SHA256 `a69f8d1a69f1ce48664c0653d5fed6f4c92f64651166923543151d3a27a570d5`; raw log in relay1-playwright.log. RED Foundation `40858325259` / job `123394865334` succeeded; that is not implementation acceptance.

Root cause: both the manual handler and disabled control reject the old persisted session boundary forever. Candidate repair enables manual recovery and requires the localized native confirmation below only when the pending attempt belongs to an earlier session. Automatic recovery still rejects that boundary. Acceptance sends the original key/body once through design-client's current-session and CSRF fence; server authentication/store scope remain authoritative. Cancellation sends nothing and changes no journal bytes. The old boundary remains in the journal, so another ambiguous answer or reload still requires explicit consent. No boundary rebinding, new key, draft substitution or list-derived success.

P2 storage-write failure remains **P2 for issue** in the PR comment, exactly as dispatched; not fixed here. Targeted P1 implementation attempt 1; CI and independent acceptance pending.

## Acceptance gate and current click ledger

The six unchanged RED cases must pass on this implementation head, together with all existing PR gates. No test, threshold, fixture, retry, timeout, baseline, dispatcher table or gate-adapter change. No new imports; component harness dependency shape unchanged. Go dependency graph unchanged; depmap regeneration not applicable. Mandatory browser ownership is regenerated last before push.

| Page/control | Action | Expected visible/persistent result | RED actual / candidate |
| --- | --- | --- | --- |
| Design in zh-TW, zh-CN, en | Lost committed/uncommitted publish answer | Original identity retained, pending and Publish locked | RED reached / GREEN pending |
| Account sign out → identity sign in → Design → reload | Real clicks/navigation with rotated auth | No automatic replay; original journal unchanged; recovery enabled | RED enabled assertion failed in all six / GREEN pending |
| Recovery confirmation | Cancel | Localized warning; no POST, journal unchanged, Publish locked | RED not reached / GREEN pending |
| Recovery confirmation | Accept | Same key/body, exact receipt and source version, pending clears | RED not reached / GREEN pending |
| Reload → edit → Save draft | Real reload/fill/click | Pending stays cleared and Publish becomes usable | RED not reached / GREEN pending |

## Three-language copy

Existing typed designCopy locale entries are used; there are no hard-coded component strings. Browser cases cover the main Design screen and confirmation in all three locales.

| zh-TW | zh-CN | en |
| --- | --- | --- |
| 確認先前草稿 v{n} 的發布請求？如果尚未完成，這會將該草稿發布給買家。 | 确认之前草稿 v{n} 的发布请求？如果尚未完成，这会将该草稿发布给买家。 | Confirm the earlier publish request for draft v{n}? If it has not completed, this will publish that draft for buyers. |

## BOURDIEU

Q1/Q2/Q3/Q4/Q5: retain the existing screen decisions; this fix introduces no new audience classification or layout. Both recent-task novices and experts use the same original-request recovery; the warning names the draft and buyer-visible consequence. Q6 aesthetics: UNKNOWN until owner. No device/class inference or aesthetic verdict.

## PROJECT UNDERSTANDING

Read architecture §§0/1/2/26/27 and storefront-v2 R1–R6. §0 separates evidence from claims; §1 keeps publication authority in Go/PG; §2 keeps business/store boundaries; §§26–27 require isolated work and independent acceptance. R1/R2 establish transactional receipt identity; R3 requires identical resend and bounded automatic attempts; R4 terminal outcomes clear pending; R5 preserves the original attempt across reload. The current review requires an explicitly authorized authenticated recovery after login, while preserving automatic fencing.

## MOBBIN → EAGLE

No redesign/reference import: existing Design layout and platform-native confirmation pattern retained. No new image, brand or product facts, and no missing visual pattern query. Existing reference provenance remains in the parent delivery.

## FIGMA OFFICIAL GUIDANCE

No Figma edits, frames, prototypes, tokens or export; native browser confirmation adds no designed component. Auto-layout/component/variable/prototype guidance and Figma lint are not newly claimed. Desktop/mobile fields and summary facts remain unchanged; every new visible string is in the three-language table above.

## Scope, rollback, evidence and wakeup

Authored production files: apps/admin/components/Design.tsx and apps/admin/lib/design-copy.ts (same Design module). Retained tests-only spec was already listed outside original write_paths; mandatory generated browser ownership plus relay1 raw evidence are listed in the PR body. Existing unrelated dirty parent DELIVERY and all old artifacts remain untouched. No contract, migration, OpenAPI, pin, dependency or lockfile edits.

Rollback: revert the implementation commit alone; retain the RED regressions. Local node --check on the regression spec and design-copy.ts and git diff --check exited 0 (E1 syntax/whitespace only). tsc NOT_RUN: no local TypeScript dependency/command available; no dependency installation. Author assessment E0. GREEN and independent review PENDING. No local tests/server/browser/PG/build/container, no LIVE/provider/deploy/Figma. No merge/auto-merge/status posting. New CI run/job IDs will go into a PR comment with integrator relay as wakeup; no evidence-only push after GREEN. Humaux fix record `59c954e8-5156-4c25-9a6c-f416d6c155ba`.

---

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

Fresh CI handoff: head `fbaec2b35f5bd9504035c0ca81a41a9f07edf5df`; Gates `40858325183`, Foundation `40858325259` / job `123394865334`, both pending at first read. New assertion RED and GREEN NOT_RUN/PENDING. These local receipt additions are intentionally uncommitted to avoid restarting CI. PR comment carries the remote handoff; integrator completion relay is the wakeup.
