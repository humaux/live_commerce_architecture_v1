## FIX5 / 2026-10-29 — authorized recovery-oracle migration (CI pending)

Authority: the current integrator FIX5 dispatch for #221; diagnosis `output/integrator/tools/logs/cxf-diag-221.log` (read-only main checkout); frozen `contracts/storefront-v2.md:396–406` R1–R6 and current implementation. Architecture sections 0/1/2/26/27 read. This section supersedes the older oracle-authorization blockers below; historical entries remain evidence, not current status. Implementation b11016111 stands; this change only migrates its browser acceptance at equal strength.

Base before work: d4a2c73ab3995ea5d800dee1e10e8baf1fdd5b4a; merged origin/r3/integration d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e first, merge 059a0afdc. Only conflict: generated browser ownership; resolved by `node scripts/dev/derive-browser-ownership.mjs`, never hand-edited. Assigned branch `unit/oc11-publish-pending-offline`, worktree `oc11`, role dispatched implementation worker; runtime Codex / GPT-6 family, effort not exposed. No recursive delegation or separate claim.

### Old → new oracle checklist (original spec line numbers)

| Original oracle / fixture | FIX5 replacement and retained strength |
| --- | --- |
| :41–44 after-commit transport | While cut, every successful server answer is observed by the transport fixture then aborted to the browser; pending, disabled Publish and no success remain. Only after transport is restored can identical-key/body POST deliver the stored original VersionInfo. All observed server receipts and recovered version must agree. |
| :48 before-commit transport / :108–127 | Requests remain aborted before Go while down, including across reload and manual confirmation. Failed or readable versions display never establishes an outcome. Restore transport: the same original key/body commits once; receipt source_version and exactly one published row are checked, then resolved storage stays clear after reload. |
| :64 recheckSettled; :224/:249 callers | Wait for the real recovery POST request and its response/failure, then the busy control settling. No versions GET is required to resolve. GET remains only post-receipt display/row-count evidence. |
| :90 published expectation | First prove continued pending while the original receipt is unheard. Restore transport, assert 200 and exact returned version in the visible success note. |
| :98 idempotency_key on VersionInfo | Frozen DTO unchanged. Correlate original Idempotency-Key header + exact body with recovered POST response; compare its full VersionInfo with the original lost receipt and its exact history version. Preserve the single-published-row assertion. |
| :122 not-yet / :143 old row / :205 stale list | No not-yet/list inference while POST remains cut; pending, no success and disabled Publish retained. Old same-draft publish/rollback and stale-list counterexamples still execute via real clicks. |
| :224 other tab | Another tab really publishes the same draft; A's original POST remains aborted. Manual same-key resolution stays pending and cannot show B's success. Original key/body identity assertions retained. |
| :249 external rollback | Pending still blocks local rollback. A different tab really rolls back. After automatic recovery settles, restore transport and receive A's original stored receipt; only then display superseded, unlock Publish and verify resolved storage after reload. Wait for post-receipt supersession display before sampling the existing independent predicates. |
| :268–275 hung read | Hold the recovery POST itself after automatic retries settle. Observe its original header/body, real click, disabled busy control and actual five-second timeout. Existing `toBeEnabled({ timeout: 7000 })` unchanged; additional elapsed >=5000ms check prevents an immediate unrelated GET failure from passing. Pending, disabled Publish and no success retained; release in finally. |
| :294 zh-TW / zh-CN / en | In each locale, pending survives reload and aborted POST recheck without success/list-derived not-yet. Restore transport: original-key/body receipt yields the localized success version; resolved pending stays clear on another reload. |
| R4 stale draft counterexample | Added a real second-tab draft edit/save while the original never-committed request is pending. Restored POST must keep original key/body/version, return 409 conflict and visible draft-changed feedback, clear pending without publishing the newer draft, and remain cleared after reload. |

All existing scenarios remain; no DTO, production source, timeout increase, test retry, threshold, fixture exemption or gate-adapter change. Every interaction uses click/fill/reload. Existing read-only sessionStorage measurement and in-flight transport fixture are retained. No page.evaluate mutation, forced click or direct API substitute for UI acceptance.

### Evidence and click ledger

Historical assertion RED: tests-only eec62d477 → Gates 40785921695 / Go shard:g09 job 123162345931 (`TestStoreDesignOC11PublishReceipt`); browser job 123162345030. Most recent pre-FIX5 oracle RED: PR head 9228365da9b27c4f1e99a1d5952fe9e62f28758d → Gates 40814762280 / bundle:04 job 123254463571, synthetic ff3e587b02d5ab7b7733ea64c27e764cd0c3b97c. Official artifact 12871899297: 9 failed / 10 passed; all eight original SDB tests pass. Original playwright.log SHA256 47fc9a904925efd3506fb4f3500231b8630a50a6c377c11fd7a90c4f0f5c7b37. Fresh failed-job read in fix5-prior-failed.log. Historical Foundation 40814762081 / 123253219083 success is partial evidence only. RED precedes implementation; no duplicate RED or mutation of the accepted implementation.

| Page / control | Action and required visible result | Candidate actual |
| --- | --- | --- |
| Design / Publish + Confirm | Lost answer → pending; same-key/body recovery → exact published version, one row | NOT_RUN; CI pending |
| Design / Reload + Confirm (three locales) | Pending survives reload, offline remains pending, online receipt resolves, cleared state survives reload | NOT_RUN; CI pending |
| Design / Versions + Rollback, two tabs | Local rollback blocked pending; external rollback followed by original receipt shows superseded | NOT_RUN; CI pending |
| Design / Save in second tab | Newer draft makes original pending request return conflict; never success | NOT_RUN; CI pending |
| Design / Confirm with hung POST | Busy disabled, five-second timeout releases within unchanged 7000ms margin, still pending | NOT_RUN; CI pending |

Static only: Node v24.15.0 `node --check tests/admin/oc/design-publish-pending.spec.ts` exit 0 (E1 syntax only); ownership generator exit 0. `git diff --check` initially found a pre-existing blank EOF in dirty DELIVERY.md; the new current section is prepended and final whitespace validation follows. tsc NOT_RUN: worktree dependencies are absent. No local tests/builds/servers/PG/browser/containers. No new Go import; Go dependency map unchanged. Author assessment E0. Current-head GREEN, independent review and LIVE remain NOT_RUN/PENDING.

Scope: authored test `tests/admin/oc/design-publish-pending.spec.ts` + delivery/red evidence under `output/oc-11/`; outside named spec/fixtures only mandatory generated `output/ci-select-backend-browser/browser-ownership.json`, listed for integrator approval. Other merge-arriving changes are integration content. Pre-existing dirty DELIVERY additions and all untracked historical evidence are preserved; only this section is staged by FIX5. PR head/run/job IDs are posted in a PR comment after the one batched push, never an evidence-only follow-up commit. Wakeup: integrator relays fresh Gates/Foundation conclusions in a new session; no silent background poll/process.

# OC-11 continuation — 2026-10-29 current implementation candidate

This section supersedes the historical delivery/status sections below. Authority: dispatched CARD 221-cont and frozen `contracts/storefront-v2.md:396-405` at starting head f78a3977cc00f9e081efc1b625bfbdcca3b40736. `cards/cxf-221-cont.md` is absent. Role: dispatched implementation worker, Codex; actual model/effort not exposed by runtime. Worktree: oc11; branch: unit/oc11-publish-pending-offline. No recursive delegation.

## R1–R5 changes and remaining acceptance conflict

- R1/R2: HTTP validates exactly one well-formed key with storefrontDomainKeyed. command.Run wraps design.Publish in the existing scoped transaction, operation design.publish. The receipt conflict is mapped to 409 idempotency_conflict separately from the callback's 409 draft CAS conflict. Existing five Go receipt subcases remain verbatim.
- R3/R4: the editor persists before the initial POST, sends at most three automatic identical-key/body recovery requests with exponential backoff, then one request per merchant confirmation click. It never infers success from a versions list. 200 resolves and shows the returned version; conflict resolves to the localized draft-changed message; idempotency_conflict retains pending and shows an integrity message. Ambiguous answers retain pending. Publish/local rollback are disabled while pending; editing remains available. A later version is shown as superseding only AFTER a successful receipt, using a bounded display-only versions read.
- R5: sessionStorage retains key, original draft version, session fence, and automatic retry count. Reload restores pending and resumes the same request without resetting the automatic budget. A session fence mismatch cannot replay another session's attempt. Pending storage is cleared only on a terminal R4 outcome. Storage-unavailable browsers fall back to memory; reload durability in that environment remains a limitation.
- Suite: OC reauth selects oc/reauth.spec.ts; the design spec stays in store-design under its actual fixture. Existing publish callers in Go fixtures receive fresh synthetic keys; no existing expected result, fixture data, threshold, timeout or exemption is relaxed.
- AUTHORIZED old → new oracle: the eight publish transmission-count assertions now check the number of distinct attempt keys, a valid key on EVERY transmission, and identical bytes for EVERY retry under that key. Added first-journey assertion: exactly one publish version row for the original draft. Existing reload/old-row/other-tab/supersession/bounded-read assertions remain.
- BLOCKED acceptance compatibility: the ruling says only the transmission-count oracle may be rewritten. The unchanged browser spec also requires GET recovery, a key in versions.items, and a failed-GET-versus-empty-list distinction. R1–R5 define keyed POST recovery and no versions-key DTO. These remaining non-count assertions were NOT changed. An asynchronous clarification asks authorization for equally strong receipt-based replacements. No approval received at preparation time; no GREEN claim.

## Evidence and commands

- Existing causal RED: tests-only eec62d477; head 75a31f92255281c232bc84a8276b03e9d0f09d78; Gates40785921695, jobs123162345931 (Go replay/atomicity, different body, missing/duplicate key, concurrency),123162345030 (six design browser failures),123162344940 (suite collection failure, NOT assertion RED). Historical RED receipts remain in red.log; not current acceptance.
- Starting-current-head readback: gh pr checks221 returned pending (exit8), Foundation40811863742/job123243793356 and Gates40811863847/build123244033318; no failed current-head job/artifact existed to inspect. Old-head artifacts were not treated as current status.
- git fetch origin && git merge origin/r3/integration: exit0, Already up to date.
- gofmt on changed Go sources/fixtures: exit0. go build ./internal/httpapi ./internal/design: exit0. go vet ./internal/httpapi ./internal/design: exit0. These are E1 static evidence only.
- pnpm --filter @live-commerce/admin typecheck: exit1 before checking, tsc not found (worktree has no node_modules). No dependency installation or server launch.
- python3 scripts/check_packet.py: exit0, PASS_PACKET_STRUCTURE_ONLY; incidental generated timestamp restored (no output outside scope retained). node --check tests/admin/timezone-ui.test.mjs and bash -n scripts/dev/depmap.sh: exit0. git diff --check: exit0.
- node scripts/dev/derive-browser-ownership.mjs: exit0, generated browser-ownership.json (53 modes). bash scripts/dev/depmap.sh: exit0, no dependency-map content diff.
- Component import audit: rg -l 'Design\.tsx' tests/ finds tests/admin/timezone-ui.test.mjs; existing design-client mock already covers the removed named import; no new import/global needed.
- Local tests/PG/browser/servers: NOT_RUN (prohibited). New-head CI, independent review, production: NOT_RUN/PENDING. Author assessment E0; no completion/E3 claim. Final run/job ids go in the PR comment, never an evidence-only push.
- Humaux/coord/code-index tools unavailable. No shared memory/canvas/lock write claimed. No spawned process remains.

## Scope accounting

Implementation: internal/httpapi/design.go, apps/admin/components/Design.tsx, apps/admin/lib/design-client.ts, apps/admin/lib/design-copy.ts. Tests: internal/httpapi/design_test.go; tests/admin/oc/design-publish-pending.spec.ts; tests/foundation/store_design_test.go, store_design_gate_test.go, browser_storefront_test.go, browser_click_sweep_test.go (publish-call fixture compatibility only). Coordination/generated: playwright.config.ts (explicitly authorized suite correction), output/ci-select-backend-browser/browser-ownership.json. Evidence: output/oc-11/**. List the broad browser fixture paths for integrator scope review. No migration, R2 pin, contract, baseline, lockfile, dependency or other backend package edit.

## BOURDIEU

Q1–Q5: UNKNOWN; no owner/source verdict was available through Humaux. This continuation fixes receipt behavior and keeps the existing page structure. Q6 aesthetics: UNKNOWN until owner. No class/device expertise assumptions or new design verdicts.

## PROJECT UNDERSTANDING

Read 架构.md sections0,1,2,26,27: distinguish evidence tiers; Go/PostgreSQL single transaction authority; server tenant scope; isolated worktree/card ownership; independent CI acceptance. Read storefront-v2 section B and Continuation R1–R5: draft CAS, atomic receipt replay, bounded recovery, reload durability. Owner/integrator ruling: preserve reload oracle and replace transmission-count oracle only. No invented settings or business facts.

## MOBBIN → EAGLE

NOT_RUN: no new visual design/reference import in this behavior continuation. No reference image/brand/fact copied. Humaux/audit candidate authority and module Eagle folder unavailable; no invented candidate IDs or reference claims. A needed query for a future design card: pending publication outcome / resumable idempotent confirmation. Integrator owns imports.

## FIGMA OFFICIAL GUIDANCE

NOT_RUN: no Figma canvas authored or handed in in this continuation; no bridge writes, prototype/copy-lint/parity completion claimed. Existing design screenshots are historical and not current acceptance. No new layout/components/variables/prototype pattern introduced. Current work requires CI real-click acceptance, not visual certification.

## Required field parity

| Publication fact/control | Existing desktop/mobile shared component | Review/pending state |
|---|---|---|
| Original draft version | Design toolbar and localized pending body | retained with key; never replaced with newer draft |
| Pending result | same responsive pending component | Publish disabled, confirmation available after bounded resolution |
| Published version | receipt result note, live indicator | later supersession shown separately |
| Changed draft / integrity | localized result text | changed draft clears pending; integrity retains it |

This is a source inventory, E0; exported-frame/browser parity and density probe are NOT_RUN. No Figma design hand-in claimed.

## Copy (current continuation)

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| publishPending | 發布結果待確認 | 发布结果待确认 | Publish result pending confirmation |
| publishPendingBody | 正在確認草稿 v{n} 是否已發布。你可以繼續編輯；確認結果後才能再次發布。 | 正在确认草稿 v{n} 是否已发布。你可以继续编辑；确认结果后才能再次发布。 | We are checking whether draft v{n} was published. You can keep editing; confirm this result before publishing again. |
| publishRecheck | 確認發布結果 | 确认发布结果 | Confirm publish result |
| publishRechecking | 確認中… | 确认中… | Rechecking… |
| publishUnconfirmed | 暫時無法確認結果，請稍後再次確認。 | 暂时无法确认结果，请稍后再次确认。 | The result could not be confirmed. Try confirming it again. |
| publishChanged | 草稿已變更，請載入最新草稿後再發布。 | 草稿已变更，请加载最新草稿后再发布。 | The draft changed. Reload the latest draft before publishing. |
| publishIntegrity | 本次結果與原請求不一致。請保留此頁面並聯絡支援人員。 | 本次结果与原请求不一致。请保留此页面并联系支持人员。 | This result does not match the original request. Keep this page and contact support. |
| publishSuperseded | 本次發布已成功，目前已由更新版本取代。 | 本次发布已成功，目前已由更新版本替换。 | This publication succeeded. A later version is now live. |

## Click ledger (CI pending)

| Page/control | User action | Expected | Actual |
|---|---|---|---|
| Design / Publish | click after fill | durable original identity, bounded same-key recovery | NOT_RUN on candidate |
| Design / Confirm publish result | click after automatic budget | one identical request, no replacement key | NOT_RUN on candidate |
| Design / refresh | browser reload | pending retained; same identity/version/budget | NOT_RUN on candidate |
| Design / edit while pending | fill + save | edits allowed, unresolved attempt unchanged | NOT_RUN on candidate |
| Design / rollback while pending | inspect disabled control after tab click | cannot race unresolved publish | NOT_RUN on candidate |
| Design / locale zh-TW, zh-CN, en | navigate and click | localized pending/recovery state | NOT_RUN on candidate |

## Candidate file hashes

- apps/admin/components/Design.tsx: 5d122757d9851e4366b7672c9e60069d18782f3b5b44973ac9234368f4c9dfd7
- apps/admin/lib/design-client.ts: 30a1b159da8895ca4674f60f9dd20392df6d15228726ba1c0ad6e17f79f73b5d
- apps/admin/lib/design-copy.ts: 9e46c4d65d19bc292e89d36e0d72c9b232aff061a05395299bffb29b75246128
- internal/httpapi/design.go: dba595ca7a504968173cc8c1a9da4ab0bab68370c85bd485fd664c4321694bcc
- tests/admin/oc/design-publish-pending.spec.ts: 1b770e06cfed4fdf583437f9631eea20cbafc8216181b489afdcc7fdbcf674a8

---

# OC-11 delivery (ui_worker, base r3/integration 33a03bbb8)

Figma (read-only): SF-04 940:6524/6525 (the "publish status" page; the pending state itself is built from the owner text OC-Q10), SF-06 1253:17180 (desktop) / 1253:17204 (mobile). Screenshots: output/oc-11/figma/.
Deviation from Figma: node 1253:17204 says "你已填寫的內容還在"; OC-Q18 says promise no input persistence, so the copy omits it.

## Backend recheck finding (read from code)
- Go `POST design/publish` (internal/httpapi/design.go -> design.Publish, internal/design/store.go) does NOT use Idempotency-Key and has no receipt table / ops.command_results row.
  Safety is compare-and-set on `expected_draft_version`; a replay after commit is 409 `conflict` (newest row is a `publish` with source_version == version).
- A read path exists: `GET design/versions` rows carry `kind` + `source_version`. A committed publish of draft vN is exactly a `publish` row with source_version=N.
- Implemented: uncertain (network error / 5xx / unreadable 2xx) -> pending state, key + version kept, publish button disabled, recheck = GET versions only (`publishLanded`). Found -> success note + live version; not found / read failed -> stays pending, never success. No new publish, no new key.
- Limit: "not found" cannot distinguish "never arrived" from "still in flight"; the UI says so and only offers recheck. No resend path exists (owner may later want a same-key replay; safe by CAS but not built).

## Storefront
error.tsx: single read-only reload (router.refresh + reset, GET only), offline banner via useSyncExternalStore on navigator.onLine + online/offline, title focused on mount, role=alert/status, buttons 48px, no persistence promise.
Copy keys added (admin: publishPending*, publishRecheck*, publishNotYet; storefront: reloadPage, offlineBanner) in en/zh-TW/zh-CN.

## Round-2 ruling (coordinator)
- OC-Q18 scope: it restricts what the error card OFFERS as recovery (only the read-only reload; never a replay of a payment or order). Shop header/footer navigation (GET links, search, cart drawer) is site chrome and stays; assertion B3 is scoped to the error card (`page-error`).
- Final fix: pending {key,version,watermark} is armed (sessionStorage + leave guard) BEFORE publish() is called and cleared only on a definite answer; the recheck watermark is max(list top, highest version this component confirmed), so a failed refresh after a success cannot leave a stale baseline. Tests OC11-A9/A10/A11. GREEN ids: PR comment (bound to the final SHA).

## Files
apps/admin/lib/design-client.ts, apps/admin/components/Design.tsx, apps/admin/lib/design-copy.ts (extra), apps/storefront/app/[locale]/error.tsx, apps/storefront/lib/shop-copy.ts (extra), tests/admin/oc/design-publish-pending.spec.ts (extra), tests/storefront/offline-error.spec.ts, output/oc-11/**.
Integrator-granted: tests/foundation/browser_store_design_test.go (spec added to the playwright invocation), tests/storefront/shop-real-gate.mjs (calls the check; not counted in cases=9), tests/foundation/browser_storefront_test.go (asserts the check reported; harness-only `api-down`/`api-up` control that makes the buyer API answer 503 so error.tsx renders; the app has no test route).
Needed beyond the grant (flag for the integrator): playwright.config.ts, one entry "oc/design-publish-pending.spec.ts" in the store-design testMatch (without it the spec is never selected).
Dropped: the earlier shop-gate.mjs registration (runs only under LC_SHOP_MOCK=1, never in CI).

## Evidence
- RED (deliberate mutation: recheck republishes with a new key; offline banner disabled): see output/oc-11/red.log. Admin run 40462789529 job 122056953409 failed at OC11-A2; storefront run 40502862891 job 122191823469 failed at OC11-B2. Setup errors before that are listed there and not counted.
- Review round 1 (DeepSeek + Kimi REQUEST_CHANGES at ca1be4157), all items fixed: recheck now needs the NEWEST row to be a publish of the sent draft version with version > the watermark recorded at send (the versions DTO carries no idempotency key; gap documented in design-client.ts); pending {key,version,watermark} persisted per store in sessionStorage and included in the beforeunload guard; failed recheck read has its own "could not confirm" state; error page has only the reload (no link), role=alert narrowed to title+body, neutral errorBody; setLive from list.live_version. RED ids in red.log.
- GREEN: bound to the final SHA, run/job ids in the PR comment (NOT_RUN in this file until then).
- Local: none (no node_modules). Level: E3 once green, MOCK/real-stack Next + Go harness, no device.
- Not covered: Safari/WebKit (offline events differ), real network loss mid-payment, the "still in flight vs never arrived" ambiguity of recheck.
- Risks: the admin spec uses the BFF-probe store and leaves one extra published version there; recheck cannot resend (by design).

## Directed fix 1 (#221, 2026-10-28)
- Base: deb79b87dbe933267a23a89b9e48c368dcc75556; assigned branch unit/oc11-publish-pending-offline. Role: ui_worker; runtime model: GPT-6 (exact model ID/reasoning setting not exposed). No delegation.
- Existing RED: Gates 40630002101 / job 122684528117; artifact 12812066011 error-context.md confirms the design-name fill timeout and the visible "The store design is temporarily unavailable. Try again." state. The test installed the versions-abort route before the initial asynchronous load finished, preventing Editor from mounting. This is a setup failure, not A11 assertion RED.
- Fix: move the real fill before fault injection. Product behavior and all A11 assertions are unchanged; no timeout, retry or force option added.
- Local static evidence: python3 scripts/check_packet.py exit 0 (PASS_PACKET_STRUCTURE_ONLY); git diff --check exit 0. The packet check's generated experiments/results/packet-check.json was restored to its clean starting content. No local tests or servers; no dependencies available for a local typecheck. E1 structure only; author assessment is E0. GREEN and independent review remain pending.
- Changed paths: tests/admin/oc/design-publish-pending.spec.ts (outside the original contracts/tasks.json write_paths; explicitly targeted by this directed card, flagged for integrator approval), output/oc-11/DELIVERY.md and output/oc-11/red.log. No CVS changes. Go dependency graph unchanged; no dependency-map regeneration needed.
- Acceptance: the PR's own gate --browser-design must complete the unchanged OC11-A11 assertions. Publish the fix once, record Gates/Foundation WAIT_RUN IDs in the handoff; integrator relays completion. No merge or auto-merge. Rollback: revert only this directed-fix commit on the assigned branch.

## Review fix RED dispatch (#221 / #253 / #264, 2026-10-29)
- Authority: owner dispatch in this session, independent REVIEW-221.md (reviewed head 542ae9c31413de2d645ac42b2fbafab5f395ea7b), contracts/owner-choices-20261026-v1.md OC-Q10 and current contracts/storefront-v2.md OC-Q10 amendment. Prior output receipts are historical only.
- Assigned role: ui_worker; runtime: Codex (exact model ID and reasoning setting not exposed); no subagents. Worktree: .claude/worktrees/oc11; branch: unit/oc11-publish-pending-offline; PR #221 into r3/integration. Starting head 542ae9c31413de2d645ac42b2fbafab5f395ea7b; integration merged from bf161bac96252c35337e07713c08a275d48e3e17, merge checkpoint 77627f4fd.
- RED batch contains tests plus TWO explicitly labelled minimal regression mutations in Design.tsx: arm pending only after an uncertain response; capture only the stale list watermark. These reproduce the unfixed A9/A10/A11 behavior. No implementation fix is present. Do not merge or deploy this RED head. Restore both mutations in the one implementation push after assertion-level CI RED is observed.
- RED oracles: A9 reads persisted key at the held POST boundary; A10 reloads during the hold, clicks Publish only if incorrectly enabled and waits for that request failure before asserting exactly one POST; A11 retains the original stale-list assertion and injects failure only after editor readiness. Initial-load timeouts are not accepted as RED.
- P1-a: original-key metadata is required in versions; a real second tab publishes the same draft, while the original tab loses a request before Go. It must never report the other tab's success.
- P1-b: local rollback must be disabled while pending; a second tab can roll back a committed-but-unheard publication, after which Recheck must explain supersession, clear persisted pending and enable a fresh publication. A hung read must release the Recheck control within five seconds plus two seconds scheduling margin.
- Three locales: real-click pending/recheck + reload cases for zh-TW, zh-CN, en; this is an implementation recovery fix, with no new visual design or Figma writes.
- Click ledger (all CI PENDING): Publish / save then publish / Recheck / Versions / local rollback disabled / external rollback / browser reload; visible and persisted results asserted by tests/admin/oc/design-publish-pending.spec.ts. No force, dispatchEvent, DOM modification or API action substitutes for clicks. page.evaluate only reads sessionStorage.
- Scope beyond original card: tests/admin/oc/design-publish-pending.spec.ts (explicitly assigned by this review fix); output/ci-select-backend-browser/browser-ownership.json (required generated ownership for the merged browser changes). Integrator approval remains required. No new migration number, R2 pin or protected baseline change. No component import additions.
- The existing backend has no publish receipt. Current storefront-v2 requires a frozen terminal-receipt contract before implementation. The continuation must keep UNKNOWN distinct from failure, preserve key + version + watermark, and resolve committed/superseded/definitively failed requests without a new publish write. A timeout alone cannot prove failure. No migration number was assigned: do not invent one; prefer existing scoped command receipts if compatible, otherwise escalate for an assigned migration/contract.
- Local verification: static packet/diff checks only; no local tests, servers, browsers, containers or PG. Browser ownership regenerated by the explicitly required node scripts/dev/derive-browser-ownership.mjs. Skill context script was not executed because it is outside the card's permitted static commands. Humaux memory/coord/code-index tools are absent; no store, canvas or shared-state completion claim.
- Rollback: revert the review-fix RED commit only (retain the integration merge); remove the two labelled mutations before the implementation push. CI monitoring is bounded to approximately ten minutes, then WAIT_RUN handoff to the integrator. No merge, auto-merge, status posting or production action.
- Evidence: E0 authored tests, CI assertion RED and implementation GREEN PENDING; exact IDs appended to red.log / PR, and GREEN receipts go in a PR comment without an evidence-only push.
- Static checks on the RED candidate: python3 scripts/check_packet.py exit 0 (PASS_PACKET_STRUCTURE_ONLY, E1 only); git diff --check exit 0. Restored only this run's generated experiments/results/packet-check.json. No Go imports changed, so the dependency map is unchanged.

## Relay 1: RED witness correction and contract boundary (2026-10-29)
- Starting PR head: 33bbc7e8d4f0da89ee7b28cff9c94d7f65d6873c. Current integration was fetched and merged into this branch first; the merge changes are upstream work, not new OC-11 implementation.
- Raw job API: Gates 40758862245 / bundle:04 job 123067989372 is completed/failure at that head. Checkout is synthetic merge 2d6b62b0cf35b762ccdb1917d3e170a091eef3d5 (head 33bbc7e8 into base bf161bac96252c35337e07713c08a275d48e3e17). Foundation 40758862052 / job 123065015466 passed at that head; neither that pass nor an outer bundle failure proves the new browser assertions ran.
- Issue #264's current first-hand review identifies an oracle control-flow defect: Playwright `timeout: 0` disables the assertion deadline. A soft assertion can therefore block the subsequent A10 second-click/count check and the external-rollback checks until the test times out. Replaced those zero-deadline waits with the same disabled/enabled/visible/count predicates sampled after the existing editor/recheck boundaries. No assertion outcome, fixture, timeout budget, retry or exemption was relaxed. The original two labelled mutations remain solely for corrected RED sensitivity; this is not the implementation push.
- Acceptance for this directed test fix: CI must show A9's durable-key failure, A10's actual two-POST count failure, A11's own stale-row failure and P1 counterexample failures. Setup or whole-test timeouts do not satisfy this gate. CI result PENDING at authoring; raw assertion excerpts belong in red.log when recovered.
- **Implementation STOP boundary:** current `contracts/storefront-v2.md:390` requires an integrator-frozen request-bound terminal-receipt/read contract before recovery implementation; issue #253 explicitly also requires a DTO contract change. `internal/design/store.go` Publish/VersionInfo/ListVersions and `internal/httpapi/design.go` drop the key, and published history contains no key. `command.Run` saves only successful results and is not currently called by design.Publish; it cannot prove a failed or still-in-flight request merely from absence. The card explicitly says STOP for contract changes and backend packages without ownership. Requested integrator-frozen receipt semantics and explicit ownership of internal/design + the design HTTP adapter; no contract, backend, migration or R2 pin was changed. Timeout must never be labelled definitive failure.
- Existing installed tooling was reused: gh for raw CI evidence, the mandated browser-ownership generator, and check_packet for structure only. No new tool or script. No source dependency/import changed, so no dependency-map change. Local tests/builds/servers/PG/browser remain NOT_RUN; author assessment E0; structure E1 only. Humaux/coord/code-index remain unavailable; no shared-state write or completion claim.
- Recovery: revert only the directed test-oracle commit if needed; keep the integration merge. Do not merge or deploy the labelled RED mutation. Integrator relays the new Gates/Foundation WAIT_RUN conclusions and the contract/ownership continuation; no silent CI wait is left behind.
- Recovered raw assertion evidence: official artifact 12849931133's complete playwright.log member (CRC32 5978702a verified despite later ZIP truncation) records 6 failures / 13 passes: A9, A11, original-key metadata, another-tab identity and bounded-read assertions are RED; A10 is not. P1b supersession never ran because the zero-deadline rollback assertion consumed the 30s whole-test budget. See red.log and review-red-playwright.log. Partial download was bounded, stopped and removed; no download process remains.
- A10's cancellation-delivery gap is a source inference from that result and send()'s catch path: a normal reload may reject the held fetch early enough for old code to persist pending before teardown. The test now adds a transport fixture before navigation that leaves rejected publish promises unheard until their document is discarded. It changes no DOM, storage or server facts; the original native publish requests remain route-held/aborted, and reload plus any second Publish are real browser operations. A9 still samples storage while POST is held. A10 must fail its actual second-POST count on the unfixed order; no assertion RED is claimed until the corrected CI run confirms it.
- Primary guidance checked 2026-10-29: Playwright v1.63.0 [matcher timeout propagation](https://github.com/microsoft/playwright/blob/v1.63.0/packages/playwright/src/matchers/toBeTruthy.ts#L33) preserves zero; [ProgressController](https://github.com/microsoft/playwright/blob/v1.63.0/packages/playwright-core/src/server/progress.ts#L74) only sets a deadline when nonzero. [addInitScript](https://playwright.dev/docs/api/class-page#page-add-init-script) supplies the isolated transport fixture before application scripts. Existing input/button assertions and every nonzero budget remain intact. New candidate: static packet and whitespace checks exit 0; browser ownership regenerated after the spec change.

## Relay 2: assertion evidence recovered; implementation remains blocked
- Authority: current owner dispatch and contracts/storefront-v2.md:390, internal/design/store.go:51/223/234/307, internal/httpapi/design.go:47. Read architecture sections 0/1/2/26/27, OC-Q10/Q18 and independent REVIEW-221.md. No contract override or migration allocation is present.
- Exact RED: c3d4abb0099821c0e0a7aa10bf9f932fc390b7d3, Gates 40765842298 / bundle:04 123102003431, synthetic 503296911fd8086695fba0e934fdd16cb20cc022. A9 missing durable key, A10 actual POST count 2 versus 1, A11 stale-row assertion, P1a key/two-tab identity, P1b local rollback/supersession/unlock/reload and bounded-read all fail on their own assertions. 6 failed / 13 passed; no setup timeout counted. Raw evidence: relay2-red-playwright.log; member CRC and SHA256 in red.log. Foundation 40765842391 / 123089643032 succeeded at that RED head only.
- Latest integration merged locally: c12fac9a2acafc04a7be5222c10ff2573975dde6, checkpoint fd7c8b03bc7b686c83d7d3ee62e6d3d95105c73b. No implementation or new push: frozen terminal receipt semantics, explicit internal/design + design HTTP adapter ownership, and assigned migration/R2 pin if required are missing. Timeout or an absent versions row cannot prove failure. The explicit STOP boundary prevents fabricating these contracts.
- Current local changes: output/oc-11/red.log, DELIVERY.md and relay2-red-playwright.log only, plus the integration merge. Outside-card new edits: none. Upstream merge files are integration changes. Existing PR extras remain listed above.
- Evidence classification: raw CI RED is E3 sensitivity evidence for c3d4abb0 only; this author's implementation assessment is E0/BLOCKED, GREEN and independent acceptance NOT_RUN. Local tests/builds/servers/PG/browser NOT_RUN. No code, dependencies, gates, harness, migration, pin or locale changes this relay. No new UI design or Figma writes.
- Humaux tools became available after deferred discovery; targeted memory lookup, rejected enumeration (5 representative entries, not full coverage), repo map, own canvas read and locked shared evidence storage used. Dispatched subtask: no independent queue claim or delegation.
- Download process ended at its 45-second bound; verified raw member retained, partial container removed. Resume only on integrator contract/ownership continuation; remaining RED workflow completion relayed by integrator.

## Relay 3: same contract/ownership STOP remains
- Re-read the dispatched review, repository instructions, architecture §§0/1/2/26/27, OC-Q10/Q18 and current storefront-v2 amendment. Exact authority: contracts/storefront-v2.md:390 requires the request-bound terminal-receipt/read contract to be frozen before implementation; this dispatch still prohibits contract and unowned backend changes. No frozen continuation or backend ownership extension was supplied.
- Fetched and merged origin/r3/integration dcecd6220 into this branch; local checkpoint cf5add082b9519dd628e68410ebfd96fc97139f9. This unpushed merge has the required coauthor trailer. Integration has no new request receipt contract or internal/design / design HTTP implementation. Existing local evidence changes were preserved.
- Read gh pr checks 221 and gh run view 40765842298 --log-failed --job 123102003431 (whole workflow still in progress, logs unavailable through that command); official jobs API independently confirms completed/failure at c3d4abb0099821c0e0a7aa10bf9f932fc390b7d3, Gate step failed. The job logs endpoint is readable. Foundation 40765842391 / 123089643032 completed/success at the same RED head only.
- Re-read the preserved raw browser log: A9 durable identity, A10 two POSTs versus one, A11 stale-row rejection, request-key/two-tab assertions, supersession/unlock/reload and bounded-read assertions all failed on their own assertions. SHA256 remains 4f4b85034952c6bc0a1d1ebea90bf1d7706fa07d859336e9f0d9b5558924dc82. E3 sensitivity evidence for c3d4abb0; no GREEN or completion claim.
- STOP, no implementation or push: obtain frozen terminal semantics, internal/design and internal/httpapi/design.go ownership, and migration/R2 allocation if needed. Absence and read timeout remain UNKNOWN. Do not merge the deliberately mutated RED source. No new test correction attempt or CI run.
- This relay edits only this DELIVERY.md and red.log; upstream merge changes belong to integration. Local tests/builds/servers/PG/browser, implementation GREEN and independent review NOT_RUN. Humaux/coord tools absent from available tool inventory, so no memory store/canvas update or claim was performed. No delegation or Figma action.

## Relay 4: assertion RED confirmed; continuation still required
- Current origin/r3/integration remains dcecd6220; required merge is already up to date. Local head cf5add082b9519dd628e68410ebfd96fc97139f9 remains unpushed; remote PR head c3d4abb0099821c0e0a7aa10bf9f932fc390b7d3. No extra CI push.
- Official job receipts reconfirm Gates 40765842298 / 123102003431 completed/failure and Foundation 40765842391 / 123089643032 completed/success at the RED head. Preserved raw browser log hash verified unchanged. A9/A10/A11 and both P1 counterexamples fail on their own assertions (6 failed / 13 passed), E3 sensitivity only.
- Current source authority still blocks implementation: storefront-v2.md:390 requires a frozen request-bound terminal-receipt/read contract; issue #253 requires a DTO amendment. Card ownership excludes internal/design and internal/httpapi/design.go and dispatch explicitly requires STOP for contract/unowned backend edits. No continuation or migration/R2 allocation supplied. Request those from integrator before implementation; never infer terminal failure from absence or timeout.
- Changes this relay: only DELIVERY.md and red.log; prior raw evidence retained. No implementation, tests, servers, builds, migration, locale, gate, Figma, commit or push. GREEN and independent acceptance NOT_RUN; author assessment E0/BLOCKED. Humaux targeted lookup and locked problem/canvas handoff recorded. No independent claim or delegation for dispatched work. Integrator continuation is required; prior WAIT_RUN 40765842298 is not a new implementation workflow.

## 221-cont checkpoint — Go RED pushed; browser authority conflict

Current source authority: frozen `contracts/storefront-v2.md` R1–R5, introduced in `3eedb4aa9`, with integrator route-only correction `b929716c0`. This checkpoint supersedes prior ownership/receipt-contract STOP reasons; backend publish ownership is now authorized. No migration or R2 pin is required.

Worker commit: `eec62d477b8475238b7132b48c35875981c784b5`. Five new REAL_PG Go subcases in `tests/foundation/oc11_publish_receipt_test.go` cover R1/R2 without modifying the existing browser oracles. Only `gofmt` ran locally (exit 0); commit and push exited 0. Test execution is CI-only and current assertion RED remains PENDING.

| Current frozen contract | Existing browser assertion | Required integrator ruling |
| --- | --- | --- |
| R3 resolves by identical same-key POST | spec lines 76–84 requires GET versions and total POST count = 1 | Authorize replacing the obsolete transport oracle while retaining request identity/at-most-one-commit protection |
| R3 forbids versions-list inference | spec lines 83–84 requires a key in version history | Test the original response replay instead |
| R5 pending is memory-only and disappears on reload | spec lines 95–97, 157–163 requires sessionStorage and persistent pending | Test read-only reload with no carried pending request |

No implementation can satisfy both columns unchanged. This worker stopped before production code or browser-oracle changes. Same-key payload integrity and atomic receipt/version tests remain uncompromised.

CI: worker-head Gates `40784283080` and Foundation `40784283074` were cancelled after integrator's route-only commit. Current PR head is `b929716c0e833c6bde4a7ad4a3af2c1e7df12489`; WAIT_RUN Gates `40784295574`, Foundation `40784295705`. These are workflow IDs, not plan-job IDs. No fresh assertion RED or GREEN is claimed. Integrator relays conclusions and oracle ruling in a new session.

This worker touched only the OC-11 test plus `output/oc-11/red.log` and this delivery file. The concurrent contract edit belongs to the integrator, not this worker. Evidence logs stay in the worktree for the eventual implementation commit; no evidence-only push. NOT_RUN: all local tests/servers/builds/PG/browser; R1–R5 implementation; own-head CI GREEN; independent review; LIVE. Author assessment E0.

## Relay 1 — current-head contract-drift STOP

Read `gh pr checks 221` and failed plan/required logs for current head `b929716c0e833c6bde4a7ad4a3af2c1e7df12489`; commands exited 1 (failed checks) and 0 (log reads), respectively. Gates `40784295574` failed in plan `123153575637`: `(STALE) GO_ONLY POST /v1/admin/stores/{}/design/publish`. Required `123153884075` reports the plan failure; all test gates were skipped. Raw logs are `relay1-plan-failed.log` and `relay1-required-failed.log`. This is not assertion RED.

Static source inspection (`scripts/dev/contractdrift/main.go:190-194`) shows a removed legacy finding must also leave the baseline; the integrator's absolute contract route removed the old GO_ONLY mismatch. Both the baseline and frozen contract are forbidden write paths. Per dispatch, STOP and return this reconciliation to the integrator. Zero worker fix attempts, no implementation, no source/oracle changes, no commit/push, no local tests or servers. The R3/R5 browser-oracle conflict above also remains unresolved.

Foundation `40784295705`, job `123153571312`, is still in progress at readback. WAIT_RUN: `40784295705`; wakeup is the integrator's fresh-session relay. No new workflow. Outside-card edits this relay: none. E0 author assessment; current-head Go assertion RED, GREEN, independent review and LIVE acceptance remain NOT_RUN/PENDING. Only local evidence files under `output/oc-11/` were updated; existing uncommitted checkpoints were preserved.

### Current-head correction after concurrent integrator push

Integrator commit `cfd4787146835d1ee799709f1f5e07cd1f844b04` now removes the stale baseline entry. This worker did not modify the baseline; no worker code change or push. The prior contract-drift STOP is resolved by the integrator. Historical failed job checked out synthetic merge `2f54e1347d4833c26542ae876f1e696aeabb66ed`; its failure is not current-head acceptance. Old Foundation `40784295705` is cancelled.

Fresh `gh pr view/checks` verifies current head `cfd4787146835d1ee799709f1f5e07cd1f844b04`: plan `123154800586` passed; Gates WAIT_RUN `40784683907` (build `123155070830` pending); Foundation WAIT_RUN `40784683790` (job `123154911427` in progress). New Go assertion RED and GREEN are still pending. Browser R3/R5 oracle conflict remains unresolved. Integrator fresh-session completion relay is the wakeup; no local tests/servers/builds or implementation ran. Worker changes stay under `output/oc-11/`; the integrator's baseline commit is outside worker ownership and is explicitly attributed here.

## Relay 2 — current-head ownership drift and RED wait (2026-10-29)

- Authority reread: frozen storefront-v2 R1–R5, lines400–405; existing tests/foundation/oc11_publish_receipt_test.go unchanged. Go RED is NOT_RUN at current head; Foundation's one failure was generated ownership, not a receipt assertion.
- Foundation40784683790 /123154911427 at cfd478714 failed only browser ownership drift. Excerpt: relay2-foundation-drift.log. Current integrator-pushed head75a31f92255281c232bc84a8276b03e9d0f09d78: WAIT_RUN40785921607 (Foundation job123158957762), WAIT_RUN40785921695 (Gates).
- Local merge checkpoint342bd2b6b86e240a8cdfc8feb06a8087fb80bb4b incorporates origin/r3/integration2bc6d4826; unpushed to avoid cancelling active RED. Ownership generation exit0/no diff; git diff --check exit0. No local tests/servers/builds.
- R3/R5 conflict remains concrete: tests/admin/oc/design-publish-pending.spec.ts:76–84 requires GET-only + one POST + history key; :95–97,:157–173 requires pending after reload/sessionStorage. Contract requires same-key POST resolution and loses pending on reload. Requested explicit integrator ruling; no assertion removal/weakening or implementation.
- Own new authored paths this relay: output/oc-11/red.log, DELIVERY.md, relay2-foundation-drift.log, relay2-comment.md. Upstream merged changes are not worker edits. No worker-authored paths outside scope, no forbidden edits. Local existing relay1 checkpoint files preserved.
- Humaux locked problem store queued and own subtask canvas updated; dispatched task retains integrator ownership. E0 author assessment, no completion. Integrator relays workflow completion/ruling; GREEN, independent review and LIVE remain NOT_RUN.

RELAY 3 CURRENT-HEAD TRIAGE (2026-10-29):
Remote PR head 75a31f92255281c232bc84a8276b03e9d0f09d78; local head 342bd2b6b86e240a8cdfc8feb06a8087fb80bb4b remains unpushed. Gates 40785921695 / bundle:08 job 123162344940 fails TestBrowserOC before spec collection completes. gh run view --log-failed --job refuses logs while the workflow runs, so read the completed job via gh api actions/jobs/123162344940/logs and its gate-browser-password-auth artifact 12860142604. playwright.log: Error: LC_BROWSER_BFF_STORE is required at tests/admin/oc/design-publish-pending.spec.ts:11/16. NOT assertion RED. Root: playwright.config.ts:22 oc/*.spec.ts collects OC-11 under the identity/reauth fixture, which supplies no BFF store; line35 also correctly includes OC-11 under store-design. Fixing suite ownership requires out-of-card root Playwright config; no skip/env fallback or exemption added. STOP per card; zero fix attempts on this blocker.
Existing eec62d477 already covers all Go R1/R2 RED subcases; no duplicate tests/push. Foundation40785921607/job123158957762 success at remote head, but not REAL_PG R1/R2 acceptance. Go shard jobs still pending/running; WAIT_RUN40785921695. No implementation without own-assertion RED. Frozen R3/R5 still contradict immutable existing browser oracles (GET-only/history key lines76-84; persistent reload lines95-97,157-165); integrator ruling still needed.
Evidence: relay3-collection-failed.log (raw artifact error); relay3-gate-failed.log (filtered job log). Commands: gh pr checks221 (failure/pending), gh run view40785921695 --log-failed --job123162344940 (workflow-in-progress refusal), gh api completed job logs/artifact and current PR/run/jobs (exit0), static source reads only. No local tests/builds/servers/PG/browser. No source/test/config change, commit or push. Write paths this relay only output/oc-11/**; previous uncommitted checkpoint files retained. Author assessment E0; GREEN, independent review and LIVE NOT_RUN. Wakeup: integrator fresh-session relay when Gates40785921695 finishes and suite/oracle authority is resolved.

RELAY 4 CURRENT-HEAD TRIAGE (2026-10-29):
Fresh PR/API reads confirm remote head75a31f92255281c232bc84a8276b03e9d0f09d78 and local unpushed342bd2b6b86e240a8cdfc8feb06a8087fb80bb4b. Gates40785921695/job123162344940 is TestBrowserOC collection failure, NOT new-assertion RED. Fresh artifact12860142604 confirms LC_BROWSER_BFF_STORE is required at tests/admin/oc/design-publish-pending.spec.ts:11/16; tested synthetic merge97ebc55fc2a102058647220af3d38e7a5acefcfc. Evidence: relay4-collection-failed.log. Archive removed after extraction. Root remains playwright.config.ts:22 broad OC glob plus :35 correct store-design selection; root config outside card authorization, STOP, zero fix attempts.
Frozen R3/R5 conflicts remain with immutable GET-only/one-POST/history-key and reload/sessionStorage oracles. No implementation can satisfy both unchanged; integrator must authorize strict R3/R5 oracle replacement and fix suite ownership. Existing Go tests-only eec62d477 already covers requested cases; no duplicate RED push. Go assertion RED pending Gates shards. Foundation40785921607/job123158957762 success is not REAL_PG receipt acceptance. WAIT_RUN40785921695; integrator completion relay is wakeup. No local tests/builds/servers, source edits, commits, push or new run. GREEN/independent review/LIVE NOT_RUN; author E0 only.
Commands: gh pr checks221 exit1; gh run view --log-failed --job refuses incomplete workflow; gh api logs/metadata/artifact exit0. Authored writes confined output/oc-11 (red.log, DELIVERY.md, relay4-collection-failed.log, relay4-comment.md). Prior checkpoint diffs preserved. Humaux unavailable: store/canvas NOT_RUN; no fabricated memory receipt.

### Relay 5 — current-head RED confirmed; acceptance contradiction remains

- Read every failed job on remote head `75a31f92255281c232bc84a8276b03e9d0f09d78`: Gates `40785921695`, jobs `123162345030`, `123162344940`, `123162345931`, `123183207201`, `123183927027`. Downloaded both failed browser artifacts with `gh run download`, exit 0, and inspected their playwright.log files.
- Go R1/R2 tests now have real assertion RED (job `123162345931`), replacing the prior pending assessment. Browser-design has six failed tests; browser OC is a collection error, not assertion RED. Details and exact source conflict table: [relay5-comment.md](relay5-comment.md).
- Frozen R3 same-key POST recovery conflicts with immutable GET-only/one-POST oracles; frozen R5 memory-only pending lost on reload conflicts with durable sessionStorage/reload assertions. STOP until integrator supersedes contradictory assertions or revises the frozen contract. No test/gate weakening or source implementation.
- Requested integration fetch/merge succeeded; local unpushed merge `d09613a0f735a23dd87e061d836d575168910306` includes the Codex trailer. Existing checkpoint changes preserved. Authored writes confined to `output/oc-11/**`; no new CI run or implementation push. Root suite selection is reported, unchanged.
- Author assessment E0; historical CI RED E3 applies only to remote head/synthetic merge noted above. GREEN, independent review, LIVE, local tests/builds/servers/PG/browser and Humaux store/canvas NOT_RUN. New WAIT_RUN NONE; integrator continuation is the wakeup. No task process left running; downloaded artifacts are retained failure evidence.
OC-11 relay 6: **BLOCKED; no implementation/GREEN claim (author E0).**

PR #221 remote head: `75a31f92255281c232bc84a8276b03e9d0f09d78`. Required fetch/merge exited 0 (already up to date). Local unpushed merge: `d09613a0f735a23dd87e061d836d575168910306`.

Read every failed job via `gh run view 40785921695 --log-failed --job <id>` (all exit 0), saving `output/oc-11/relay6-job-*.log`:

- `123162345931`: genuine R1/R2 assertion RED for replay/atomic receipt, changed body, missing/duplicate key, concurrent replay. Existing Go RED covers the card; no duplicate push.
- `123162345030`: browser-design, six failed tests including identity/reload/stale-list/other-tab/supersession/bounded recheck.
- `123162344940`: OC collection failure, `LC_BROWSER_BFF_STORE is required`; not assertion RED.
- `123183207201` and `123183927027`: receipt/required aggregates fail from those members.

Both browser artifacts freshly downloaded with `gh run download 40785921695 -n <name> -D <dir>` (exit 0), and actual playwright logs read. Retained under `output/oc-11/relay6-artifacts/{design,password-auth}`. Tested synthetic checkout `97ebc55fc2a102058647220af3d38e7a5acefcfc`; E3 assertion evidence applies only to it and the stated PR head.

**Unresolved acceptance contradiction:** frozen contract R3 at `contracts/storefront-v2.md:402` requires identical same-key/body POST recovery; spec `tests/admin/oc/design-publish-pending.spec.ts:76–84` requires GET recovery and one total POST. Frozen R5 at contract line 404 requires pending lost on reload; spec lines 95–97 and 156–173 require durable pending/sessionStorage. The card explicitly forbids removing existing RED oracles. No implementation satisfies both. Integrator must explicitly supersede obsolete predicates with R1–R5 checks, preserving request identity, at most one committed version, cross-tab/old-row counterexamples, bounded recovery, real clicks and all three locales. That ruling remains unanswered; no test was changed or weakened.

Separate collection root: OC wildcard also collects this design-only spec under a fixture lacking its store. No skip, fallback, exemption or root config edit.

Worker writes this relay: `output/oc-11/**` only; outside-card authored paths NONE. Existing dirty evidence preserved. No source change, commit, push, merge, auto-merge or deployment. `cards/cxf-221-cont.md` is absent; authority is dispatched card plus frozen contract. Downloads finished; no task process remains running.

NOT_RUN: implementation, GREEN, local-merge CI, independent review, local tests/servers/builds/PG/browser, LIVE. Humaux/coord unavailable; store/canvas NOT_RUN. **WAIT_RUN: NONE — no new push.** Resume needs explicit oracle ruling, not another relay of this completed run.

🤖 Generated with Codex (dispatched by Claude Code integrator).
OC-11 relay 7: BLOCKED by acceptance conflict; no implementation/GREEN claim (author E0).

PR #221 remote head 75a31f92255281c232bc84a8276b03e9d0f09d78. Required git fetch origin && git merge origin/r3/integration exited 0, already up to date. Local d09613a0f735a23dd87e061d836d575168910306 is the existing unpushed integration-merge checkpoint, not a new Go test commit (correction to this session's initial description). Go RED commit eec62d477 is already in the remote head.

Read every failed current-head job with gh run view 40785921695 --log-failed --job <id>, exit 0: 123162345030 (bundle:04, six browser failures), 123162344940 (bundle:08, OC collection error), 123162345931 (shard:g09, R1/R2 assertion RED), 123183207201 (receipts aggregate), 123183927027 (required aggregate). Fresh gh run download of both browser artifacts exited 0; actual playwright logs inspected under output/oc-11/relay7-artifacts. Synthetic checkout 97ebc55fc2a102058647220af3d38e7a5acefcfc. E3 RED evidence applies only to that tested head/checkout.

Frozen contracts/storefront-v2.md R3: same-key/body POST recovery; immutable tests/admin/oc/design-publish-pending.spec.ts:76-84: GET recovery, POST total remains 1, history contains key. Frozen R5: reload loses pending; spec:95-97,156-173: reload retains pending/sessionStorage. These cannot all pass by implementation. Requested explicit authorization to replace conflicting legacy predicates with strict R3-R5 oracles; no answer received. Separate collection blocker: playwright.config.ts:22 oc/*.spec.ts includes the design-owned spec without LC_BROWSER_BFF_STORE (it is correctly selected in store-design too). No skip, fallback or exemption added.

Commands: git fetch/merge exit 0; gh pr checks 221 exit 1; five gh run view calls exit 0; gh run download exit 0; source inspection only. No source/test/config edits, commit or push. Existing dirty evidence preserved. Writes this relay output/oc-11/** only; outside-card authored paths NONE. cards/cxf-221-cont.md absent; dispatch and frozen contract used as authority. Downloads completed; no background process remains.

NOT_RUN: implementation, GREEN, local tests/servers/builds/PG/browser, independent review, LIVE, Humaux store/canvas/coord (tools unavailable).
WAIT_RUN: NONE (no new push; Gates 40785921695 completed failure). Resume requires the oracle/suite ruling, not another relay of this completed run.

🤖 Generated with Codex (dispatched by Claude Code integrator).


Relay8: current remote75a31f92 RED verified (40785921695; Go123162345931). Frozen R3/R5 still contradict unchanged browser assertions. No implementation/push. See relay8-comment.md. Humaux problem stored, own subtask canvas updated under lock. WAIT_RUN NONE.


## Relay 9 — current-head correction and bounded CI handoff

OC-11 relay 9 — CI handoff; GREEN PENDING (author assessment E0).

PR #221 current head: b32cf40015d6d56a6d7c5c0bbcb5a2947557848e.
Initial head: b11016111dfbcb4d51bae1e5114831c0da9ec7fe. Required git fetch origin && git merge origin/r3/integration exited 0 (already up to date). Read AGENTS.md, architecture sections 0/1/2/26/27, and frozen storefront-v2 continuation. cards/cxf-221-cont.md is absent; dispatch supplies the card.

Read every failing job of the initially current head with gh run view --log-failed --job: Gates 40812890163 / plan 123247737838 and required 123247923418 (exit 0). Plan failed on four TOUCHED GO_ONLY design GET routes (draft, versions, media, media/{image_id}); required failed because plan failed and downstream jobs skipped. This is gate failure, NOT new-assertion RED. Synthetic checkout a97daa0d12795041d29f744d7a582a43913b5dda; base 61951609f1dd0937d7d6a8c2f8b0d2faf3d4d05d. Raw logs: relay9-plan-failed.log and relay9-required-failed.log. No current failed browser job/artifact existed to download. Historical RED remains Gates 40785921695 / Go 123162345931 / browser-design 123162345030; collection 123162344940 is not assertion RED.

Concurrent integrator correction: commit b32cf4001 (Claude trailer) added R6 absolute GET route declarations in contracts/storefront-v2.md and removed their legacy baseline rows in scripts/dev/contractdrift/baseline.json. This worker did not edit either forbidden path. Fresh PR/head/run reads verified the updated head; initial-head failure is historical and must not be called current acceptance. New head has no failed jobs at handoff. No duplicate implementation/evidence-only push.

WAIT_RUN: 40813368114 — Gates, queued plan 123248661255 (not a test job; not acceptance).
WAIT_RUN: 40813368385 — Foundation, queued foundation job 123248703415 (not a plan job).
GREEN: PENDING. Integrator fresh-session completion relay is the wakeup; do not continue polling this session.

Commands: fetch/merge exit0; failed-job reads exit0; run/head/API/source reads exit0; latest gh pr checks221 exit8=pending. No local tests, builds, static gates, servers, PG or browser executed. No worker source/test/config edit, commit or push. Own writes only output/oc-11/**; outside-card worker paths NONE. Existing untracked evidence preserved. Integrator's contract/baseline changes are separately attributed above. No task processes or temporary downloads remain.

NOT_RUN: current-head GREEN, local tests/servers/PG/browser, independent review, LIVE; Humaux store/canvas/coord (tools unavailable). Evidence trail is local output plus this PR comment, not a fabricated Humaux receipt. E3 applies only to the stated historical CI failures, not candidate acceptance.

🤖 Generated with Codex (dispatched by Claude Code integrator).


## Relay 10 — current-head out-of-scope contract drift

OC-11 relay 10 — BLOCKED by out-of-scope contract drift; no GREEN claim (author E0).

PR #221 head: b32cf40015d6d56a6d7c5c0bbcb5a2947557848e. Fetch/merge origin/r3/integration exited 0, already up to date. R1–R5 implementation is already committed in b11016111; no duplicate implementation or RED push.

Current-head Gates 40813368114 completed failure. Read EVERY failed job with gh run view --log-failed --job (exit 0):
- plan 123248661255: METHOD_MISMATCH for /v1/admin/stores/{}/design/draft (Go/BFF GET,PUT; contract GET) and /v1/admin/stores/{}/design/media (Go/BFF GET,POST; contract GET).
- required 123248930659: aggregate failure because plan failed; build/gate/receipts skipped. This is NOT assertion RED or candidate test acceptance.
Raw logs: output/oc-11/relay10-plan-failed.log and relay10-required-failed.log. The run has zero artifacts and no failed browser job; no browser artifact to download.

Root: integrator commit b32cf4001 added absolute GET-only R6 declarations at contracts/storefront-v2.md:405. Existing section B already declares GET/PUT design/draft (:116), and the Go route registry (:46–50,76–79) and BFF retain legitimate save/upload writes. Correct the contract's absolute method inventory by including PUT /v1/admin/stores/{store_id}/design/draft and POST /v1/admin/stores/{store_id}/design/media, both integration:manage, under integrator ownership. Worker STOP boundary explicitly forbids contract text changes. Removing runtime methods, changing baseline, or weakening drift detection would violate this card. No workaround attempted.

Historical assertion RED, not current acceptance: Gates40785921695, Go123162345931 (R1/R2), browser-design123162345030 (six browser failures); OC collection123162344940 is setup failure. Historical IDs are recorded in red.log; no historical result is called current GREEN.

WAIT_RUN: 40813368385 — current-head Foundation in progress, job123248703415; integrator completion relay is the wakeup. Gates40813368114 is completed failure, not a pending test run. No new push/run; no evidence-only commit.

Commands: git fetch/merge exit0; gh pr checks221 exit1; two failed-job reads and run/jobs/artifact API reads exit0. No local tests, servers, builds, static gates, PG or browsers executed. This relay writes only output/oc-11/**; outside-card worker files NONE. Prior dirty/untracked evidence preserved. No task process remains running.

NOT_RUN: current-head GREEN, independent acceptance, local tests/servers/PG/browser, LIVE. Humaux memory/coord/canvas tools unavailable; store/update NOT_RUN. Current acceptance awaits the integrator-owned contract correction plus the ensuing PR CI; no further polling this session.

🤖 Generated with Codex (dispatched by Claude Code integrator).


### Relay 10 correction — integrator advanced head during handoff

OC-11 relay 10 — corrected current-head CI handoff; GREEN PENDING (author E0).

PR #221 CURRENT head: 1c2885bfa3bdd178cafd54265b8770ff1d0cefb6. R1–R5 implementation already exists in b11016111; no duplicate implementation/RED/evidence-only push.

Initially current b32cf4001: fetch/merge origin/r3/integration exited0, already up to date. Read EVERY failed job of Gates40813368114 via gh run view --log-failed --job (exit0): plan123248661255 reported two METHOD_MISMATCH errors (Go/BFF GET,PUT draft and GET,POST media vs contract GET); required123248930659 failed from plan, with test jobs skipped. Raw logs: output/oc-11/relay10-plan-failed.log and relay10-required-failed.log. No browser job or artifact existed (artifact count0). This was gate failure, not assertion RED.

Concurrent integrator correction 1c2885bfa adds the missing absolute PUT draft and POST media declarations to R6, both integration:manage. Read the commit and independently verified local and remote PR head. Worker did NOT edit contract text, baseline, or runtime methods. The prior blocker report applies only to b32cf4001; current correction awaits CI, not acceptance by static inspection. Old Foundation40813368385 was cancelled.

WAIT_RUN: 40813706262 — Gates on 1c2885bfa, in progress; plan123249746885 is NOT a test job/result.
WAIT_RUN: 40813706245 — Foundation on 1c2885bfa, in progress; foundation123249826514.
Integrator fresh-session completion relay is the wakeup. No current-head failed jobs at readback; GREEN PENDING.

Historical assertion RED (not current acceptance): Gates40785921695 / Go123162345931 (R1/R2) / browser-design123162345030 (six failures); OC123162344940 was collection/setup failure. Recorded in red.log. No historical green claimed.

Commands: fetch/merge exit0; gh pr checks initially exit1, current exit8 (pending); failed-job/API/commit/head reads exit0. No local tests, builds, static gates, servers, PG or browsers executed. This relay writes only output/oc-11/**; outside-card WORKER files NONE. Integrator-owned contracts/storefront-v2.md change separately attributed above. Existing dirty/untracked evidence preserved. No worker commit/push/merge/deployment; no task process left running.

NOT_RUN: current-head GREEN, independent review, local tests/servers/PG/browser, LIVE; Humaux memory/coord/canvas unavailable, store/update NOT_RUN. Author E0 only; E3 historical failures apply solely to their stated heads.

🤖 Generated with Codex (dispatched by Claude Code integrator).
OC-11 relay 11 — GREEN PENDING; author assessment E0.

PR #221 current head: `efd1eb4a084341fc573ac9f860c077f9f6e2571e`. R1–R5 implementation already exists in `b11016111`; no duplicate implementation/RED push or evidence-only commit.

Read every failing job of the initially current head `1c2885bfa3bdd178cafd54265b8770ff1d0cefb6` using `gh run view 40813706262 --log-failed --job <id>` (exit 0):
- plan `123249746885`: STALE GO_ONLY entries for PUT `/v1/admin/stores/{}/design/draft` and POST `/v1/admin/stores/{}/design/media` after R6 declared these methods.
- required `123249966631`: failed because plan failed; test jobs skipped. This is a gate failure, not assertion RED.
- Raw logs: `output/oc-11/relay11-plan-failed.log`, `output/oc-11/relay11-required-failed.log`. Run artifacts total 0; no failed browser artifact exists to download.

Static root confirmation: `scripts/dev/contractdrift/main.go:190–194` rejects baseline entries absent from current findings. Integrator-owned commit `efd1eb4a0` removed the two obsolete entries while this relay was reading CI. Worker did not edit the forbidden baseline or contract, remove runtime methods, weaken tests, or attempt a workaround. Local/remote branch head independently reread after fetch/fast-forward check; they match. No failed job exists on the new head at readback.

WAIT_RUN: `40813908638` — Gates (workflow ID, not plan job ID), in progress.
WAIT_RUN: `40813908687` — Foundation, job `123250468197`, in progress.
Wakeup: integrator's explicitly promised fresh-session relay when these workflows conclude. No further polling or task process remains.

Historical assertion RED, not current acceptance: Gates `40785921695`, Go `123162345931` (R1/R2), browser-design `123162345030` (six failures); OC `123162344940` was collection/setup failure. Already recorded in `red.log`. GREEN run/job IDs: none yet.

Commands: initial `git fetch origin && git merge origin/r3/integration` exit 0; failed-log/API/head reads exit 0; initial `gh pr checks 221` exit 1, current exit 8 (pending); own-branch fast-forward check exit 0. No local tests, servers, builds, static gates, PG or browsers ran. No worker commit/push/merge to integration/auto-merge/deployment.

Worker writes this relay: `output/oc-11/**` only (delivery, red-log checkpoint, this comment, two failed logs). Outside-card worker paths: NONE. Integrator's forbidden baseline edit is separately attributed above. Existing dirty/untracked checkpoints preserved; no new fixtures, containers, ports or background processes.

NOT_RUN: current-head GREEN, independent review, all local tests/servers/PG/browser, LIVE. Humaux/coord/canvas tools unavailable; store/update NOT_RUN, no fabricated receipt. Missing card file `cards/cxf-221-cont.md`: dispatched card text plus frozen contract used as authority. E3 historical CI failures apply solely to their stated head/run; candidate completion remains unclaimed.

🤖 Generated with Codex (dispatched by Claude Code integrator).


## Relay 12 — repair the current-head architecture gate

Authority: dispatched CARD221-cont and frozen storefront-v2 continuation R1–R5; contract unchanged. `git fetch origin && git merge origin/r3/integration` exited 0 (already current). Start head `efd1eb4a084341fc573ac9f860c077f9f6e2571e`. The requested card file is absent; dispatched text is the task authority.

Read every currently failed job: Foundation run `40813908687`, job `123250468197`, `gh run view --log-failed --job` exit 0. Its sole failing assertion is the existing G-UI3/G-UI5 architecture gate: `Design.tsx: lines 831 > 0`. `findings()` emits the line finding above 800; no line waiver exists. This is gate RED, distinct from historical R1/R2 and browser assertion RED in red.log. Gates `40813908638` is still running and has no failed browser jobs at this checkpoint, hence no failed-browser artifact to download.

Root repair (targeted attempt 1): move the existing read-only snapshot loading hook into `apps/admin/lib/use-design-load.ts`; Design imports it. The loader's session fence, abort cleanup, read/error behavior and all publication/recovery logic are retained. Design falls below the unchanged 800-line threshold. All Design.tsx compilation harnesses were located; the sole one, `tests/admin/timezone-ui.test.mjs`, registers the new import in the same push. Its timestamp and unknown-import assertions stay unchanged. No gate, waiver, threshold, contract, migration, R2 pin or lockfile change.

Scope: new hook and Design are within authorized admin lib/components. `tests/admin/timezone-ui.test.mjs` is the existing Design compilation harness; listed outside the explicitly OC-11-named tests for integrator review under COMPONENT HARNESSES. No route/spec added, renamed or removed, so browser ownership regeneration is not triggered. No Go import/package changes, so dependency map unchanged. No visual layout/copy changes; existing UI process and locale delivery applies unchanged.

Commands: `node --check tests/admin/timezone-ui.test.mjs` exit 0; `git diff --check` exit 0. `./node_modules/.bin/tsc -p apps/admin/tsconfig.json --noEmit --incremental false` exit 127: worktree has no compiler binary, so TypeScript verification NOT_RUN (no dependency install). E1 syntax/whitespace only; author's acceptance E0. GitHub Foundation and required Gates remain the acceptance oracle. No local tests, builds, servers, browser or PG executed.

Existing dirty/untracked evidence preserved. Humaux read available this relay; fix stored and own sub-canvas updated under coordination lock. Exact new run/job IDs and final SHA are recorded in the PR comment after the source push; no evidence-only follow-up commit. Integrator's completion relay is the wakeup; no worker background process remains. NOT_RUN: current-head GREEN, independent review, local runtime verification, LIVE. No integration push, merge or auto-merge.
