# FEAT-BUYER-RMA-FIX2 — integrator-granted repair

## Current RELAY 1 — second directed implementation attempt, 2026-10-31

This section supersedes earlier current-state statements below. Current CI RED
head is `04dd888780f01f14462b6aea166576327ff6225a`. Mandatory fetch and merge of
`origin/r3/integration` exited 0, already up to date at `629f42152f864621a99ac2247c3baf5d751a422f`.
Existing untracked evidence was preserved. This uses the established card evidence
directory `output/FEAT-BUYER-RMA`, rather than creating a second case-only directory.
Authority: dispatched FIX2 grant; AGENTS.md; architecture §§0/1/2/26/27;
returns-v1 Amendment 0210; actual routing source and the original GitHub logs.
Role: dispatched implementer; no recursive delegation. Runtime model/effort labels
are not exposed. The only product edit is the card-owned storefront buyer BFF.

| Every current failing job | Failing assertion | Evidence | Class / root repair |
| --- | --- | --- | --- |
| Gates 41109119625 / plan 124246578308 | Two `BFF_NO_GO`: GET `return-requests/{}/withdraw`, POST `return-requests/{}`; two `METHOD_MISMATCH` for the same paths, `errors=4` | `FIX2-relay1-plan-red.log`, raw `gh run view --job 124246578308 --log-failed` | (b) Combined create/item branch uses nested conditions the finite BFF reader cannot pair correctly. Separate literal POST create branch; item branch uses neighboring payment route's `capture ?? "detail"` plus `allowed` GET/POST shape. |
| Foundation 41109119831 / 124246577086 | Contract-drift step repeats exactly the same four errors, `errors=4`; exit 1 | `FIX2-relay1-foundation-red.log`, raw `gh run view --job 124246577086 --log-failed` | (b) Same product declaration root; no separate test or inventory failure appears in this failed step. |
| Gates 41109119625 / required 124248826936 | `plan=failure build=skipped ... gate=skipped ... receipts=skipped`; exit 1 | Raw required job log | (b) Dependency propagation; no independent assertion changed. |

The prior dispatcher freeze failure is absent from both current logs: its earlier
repair is retained. Go route inventory remains exactly 46 + four RMA routes = 50;
R2 remains base139 + assigned0210 = 140. No further pin, contract, extractor,
allow-list, baseline, fixture, threshold, exemption or test edit. No class (a)
UI rewrite or class (c) base exemption. Browser jobs/receipts/aggregate skipped;
no standalone receipts failure or browser artifact requires a separate repair.
The new declaration preserves exact path/method admission, UUID checks, body types,
request/order binding, signed authority, response validation and no mutation retry.
Existing BFF tests exercise create/list/detail/withdraw and hostile inputs;
CI contract drift supplies the directly observed RED assertions for this repair.

Causal tests-only RED was re-read: Foundation41093678360/job124193777832, head
be932cbb748c3c88a1044ffcf242c6e04333a8b4, new BFF assertions 404≠201 and 404≠503.
See red.log; current failed CI receipts are appended there as supplemental evidence.

Local static checks: `node --check apps/storefront/lib/buyer-server.ts`, ownership
derive + `--check`, depmap generator + `--check`, and `git diff --check`: exit 0.
Generated ownership/depmap files have no delta. No local tests, server, scanner,
DB, container or browser. tsc is unavailable on PATH; full
TypeScript/build and behavioral verification remain CI-only. No UI design change.

This is attempt 2 after the pin repair, in one combined implementation push.
If the same assertion stays red, STOP with its exact receipt; no third approach.
Gates/Foundation new run IDs will be recorded in a PR comment and final WAIT_RUN;
integrator owns the completion relay, not a local unattended polling process.
No merge, auto-merge, shared-branch push, force-push, status write or deployment.
Outside-card edits in this relay: none beyond required card evidence. Previously
granted table entry/count and candidate are unchanged; independent reviewer must
still approve the entire buyerhttp.Routes closure at the new PR head.
E1 static readiness only; current CI failure is E3 for 04dd8887; author judgement E0.
GREEN PENDING. NOT_RUN: local tests/servers (prohibited), new-head CI GREEN,
Go/REAL_PG/browser acceptance, independent review E4, external SANDBOX/LIVE,
production rollout. No completion claim.

## Current disposition — 2026-10-31, supersedes historical STOP below

PR #395, `unit/feat-buyer-rma` → `r3/integration`. The latest explicit integrator
grant resolves the protected-pin boundary: only buyerhttp.Routes may be re-pinned,
with candidate justification and independent PR review; the exact inventory pin
may increase by the number of added routes. No recursive delegation.
Role: dispatched implementer. Runtime serving model/effort identifiers are not
exposed. Mandatory integration merge includes base
`629f42152f864621a99ac2247c3baf5d751a422f`; original local work/evidence preserved.

| Every failing job | Failing test / assertion | Original evidence | Class / repair |
| --- | --- | --- | --- |
| Foundation 41096837295 / 124204663270 | `TestGoRepositoryInventory`: `internal/buyerhttp/routes.go rows=50 want46` | Original `gh run view --job 124204663270 --log-failed`, filtered in FIX2-current-red.log | (b) Exact inventory pin 46 + four new routes = 50; all row/provenance and unresolved-route assertions retained verbatim. |
| Gates 41096837305 / plan 124208924912 | `dispatcher freeze`: buyerhttp.Routes `aff6d247… want bfbb4441…`; existing cart/catalog then report `BFF_NO_GO` | Original plan log, filtered in FIX2-current-red.log | (b) Changed dispatcher closure withheld the whole table. Re-pin only the authorized entry, with two added functions and one entire new const block. Existing literal route shape already matches neighbours, so no product rewrite is necessary. |
| Gates 41096837305 / required 124210208735 | `plan=failure build=skipped ... gate=skipped aggregate=skipped ... deploy=skipped ... receipts=skipped` | Original required log, filtered in FIX2-current-red.log | (b) Dependency propagation; no independent assertion to change. |

No class (a) UI rewrite or class (c) base exemption is taken. Foundation artifacts
API returns zero artifacts; no browser suite ran. Receipts/aggregate/gate skipped;
there is no isolated receipts assertion. Historical base Gates 41105181453 was
still in_progress when queried; full base GREEN is not assumed.

Candidate: `output/buyer-route-table/FEAT-BUYER-RMA-FIX2-pin-candidate.md` lists
old/new SHA and every changed/added closure function/declaration. The whole buyerhttp
package is unchanged from the original failed CI head d699e74e, so its printed
fingerprint remains the candidate. No scanner/extractor, allow-list or baseline
was run or changed. The plan computes route inventory in CI; no separate route
inventory output file requires hand editing. Independent review is pending.
R2 remains base 139 + assigned 0210 = 140; no newer 021x migration exists in this
fetched base and no migration/R2 edit is needed.

Static commands, each exit 0: gofmt on the inventory test; go vet on
`./scripts/dev/contractdrift ./internal/buyerhttp ./internal/returns ./internal/httpapi`;
browser ownership derive + --check; depmap generator + --check;
`python3 scripts/check_packet.py` (structure only); `git diff --check`.
Generated ownership/depmap have no delta; the packet-check timestamp-only output
was restored. No local tests, servers, DB, containers or browser.

Existing causal tests-only RED remains Foundation 41093678360/job124193777832,
head be932cbb: new BFF assertions 404≠201 and 404≠503; re-read from GitHub this
session. See committed red.log. This directed repair uses one combined push;
current-head GREEN is PENDING. If the same assertion remains red, stop with its
exact CI evidence under the two-attempt rule; do not take a third approach.

Out-of-original-scope changes: the expressly granted buyerhttp.Routes entry in
`contracts/route-table-dispatchers.json`, the exact count line in
`scripts/dev/contractdrift/go_extract_test.go`, and the prescribed candidate/report
evidence files. Independent reviewer must check the closure before acceptance.
No shared branch push, merge, auto-merge or commit status. Workflow IDs follow
in the PR comment/final WAIT_RUN handoff, with integrator relay as wake-up owner.

Evidence: E1 static readiness; historical RED is E3 failure evidence for its
recorded SHA, author judgement E0. NOT_RUN: current-head CI GREEN, REAL_PG,
browser/UI, independent closure review, external SANDBOX/LIVE, production rollout;
local tests/servers prohibited. This is a CI handoff, not a completion claim.

## Historical blocked relays — superseded by the grant above

Date: 2026-10-31. PR #395 / `unit/feat-buyer-rma` into `r3/integration`.
Dispatched implementer; no delegation. Model/effort identifiers not exposed by this runtime.
Earlier relays had no Humaux tools. This revalidation discovered the tools after an initially empty inventory; memory search/grep/get, rejected enumeration, repository map and canvas read now succeeded. This is an integrator-dispatched lane; no separate task claim or delegation.
Authority: dispatched FIX2 card + current AGENTS.md, architecture §§0/1/2/26/27, buyer-routes-v1, returns-v1 Amendment 0210, current routing/scanner source, original GitHub job logs.

## Current state and evidence boundary

Latest revalidation: 2026-10-30T18:31Z (2026-10-31 Asia/Shanghai). All three failing jobs were re-read directly; the classification below is unchanged. Browser ownership regeneration and `--check` were repeated (54 modes; no generated delta), as was `git diff --check`. No source/test/contract changes, commits, pushes or new workflows were created.

Humaux write receipt: the task-specific memory lock was acquired, but `memory_store`, lock release, and the subsequent canvas-lock request returned HTTP transport errors for `https://humaux.com/memory/mcp`. Store outcome is unconfirmed; canvas update was not attempted without an acquired lock. No successful store/update is claimed. The acquired memory lease expires at 2026-10-30T18:34:21Z. This local report preserves the handoff if shared-state writes did not arrive.

Merged `origin/r3/integration` **a3dc78e4d5ec72831aaf6a376793f0f6a7a56425** first. Local merged head **e48768ea69e61d7660a3cc08a4cbb3f93c2d9663**, not pushed or tested. Existing PR remote head **d699e74ea6c7c0fa3d4e4683b895b27c86f09136**.
Integration Gates run 41105181453 was re-read on this relay: head a3dc78e4d5ec72831aaf6a376793f0f6a7a56425, status in_progress, conclusion empty; plan job 124232946357 succeeded and gate shards remain in progress. Full base GREEN is NOT established. No class (c) exemption is claimed.
Historical CI assertions below are E3 for their historical SHA only. Static source diagnosis E1; author disposition E0. No current-head completion/acceptance claim.

## Every failing job → test → evidence → class

| Run / failed job | Failing test or assertion | Original evidence | Class / disposition |
|---|---|---|---|
| Foundation 41096837295 / 124204663270 | TestGoRepositoryInventory: `internal/buyerhttp/routes.go rows=50 want46`; inventory `routes=293 unresolved=78 notes=95` | `FIX2-red.log`; https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41096837295/job/124204663270 | (b) Four added endpoints require exact count 46 + 4 = 50. This count update is now authorized, but alone cannot resolve the separate frozen closure rejection. No assertion weakened or edited before STOP. |
| Gates 41096837305 / 124208924912 (plan) | Contract drift: `BFF_NO_GO` for existing buyer routes; `ERROR (dispatcher freeze) UNRESOLVED`: `buyerhttp.Routes aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05 want bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a`; log explicitly requires review and protected contract update | `FIX2-red.log`; https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41096837305/job/124208924912 | (b) PR changed the dispatcher closure, so scanner refuses the whole buyer table. Existing row literals are already correct. Requires independent full-closure review and integrator-owned pin update; protected file modification is expressly forbidden to this lane. Other NEW/TOUCHED findings remain unverified, not assumed cleared by that update. |
| Gates 41096837305 / 124210208735 (required) | `plan=failure build=skipped (expected false) gate=skipped aggregate=skipped (expected false) deploy=skipped (expected false) receipts=skipped` | `FIX2-red.log`; https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41096837305/job/124210208735 | (b), downstream of plan. No independent product assertion to change. |

Both failed runs have `total_count: 0` artifacts (GitHub artifacts API). No browser job ran. Receipts, aggregate and gate skipped; receipts did not fail alone. No signed-frame test rewrite (a), browser artifact download, or identical-base proof (c) applies.

## Root cause: the requested literal shape is already present

- Static diff against current integration: `internal/buyerhttp/routes.go` is exactly four insertions, zero deletions; all use the same `Method: http.Method*, Path: "literal", kind: ...` shape as neighbouring rows. Literal row count base 46, PR 50.
- `internal/buyerhttp/handler.go:335–337` calls `isReturnRequestRoute`; `:533–534` adds the request dispatch case calling `h.returnRequest`.
- `scripts/dev/contractdrift/go_dispatcher_freeze.go:212–227` hashes function types/bodies and traverses referenced same-package declarations; only the row data itself is excluded. Row placement/formatting cannot remove these changed dispatcher bodies.
- `scripts/dev/contractdrift/go_route_tables.go:132–139` refuses all rows before appending routes when the SHA/functions/declarations differ from the reviewed pin.
- `contracts/route-table-dispatchers.json` owns the pin and explicitly requires independent approval of the entire changed closure. It is a protected contract, not a disposable generated route inventory. No supported row-only callback exists in the current buyer `routeEntry` (Method, Path, kind); introducing callback dispatch would also change frozen type/consumer bodies.

STOP follows the card's explicit protected-file / out-of-scope gate boundary. Reformatting correct literals, reverting required product admission/dispatch, hiding the new route behind another kind, or changing the scanner would not be a valid root repair.

## Handoff needed to unblock

Integrator must obtain the contract-required independent closure review and own the protected pin amendment, or provide a revised authorized routing contract. The exact 46 → 50 count amendment remains pending for the subsequent combined fix. No third approach or CI retry was attempted. This relay used zero repair attempts and one read-only directed diagnosis; prior FIX1 reports zero repairs.

R2 base pin remains 139; this PR's one assigned 0210 migration leaves 140 correct by the assigned rule. No later 021x migration exists in this fetched base. No migration/pin edits were made.

## Static receipts / scope / follow-up

- `node scripts/dev/derive-browser-ownership.mjs`: exit 0 (54 modes); then `--check`: exit 0. Generated content unchanged.
- `git diff --check`: exit 0. No local tests, scanner execution, test servers, database or containers.
- No authored source/test/contract change; no browser ownership or dependency map delta. Only prescribed integration merge and this report plus `FIX2-red.log` were created in this relay; pre-existing untracked evidence preserved.
- No push, new workflow, merge to a shared branch, auto-merge, force-push or commit status. The integration merge is local and includes the required Co-Authored-By trailer.
- WAIT_RUN: NONE — hard stop before push. Integrator receives this explicit handoff; no background process or silent CI wait remains.
- NOT_RUN: current-head Foundation/Gates GREEN, current-head unit/Go/REAL_PG/browser acceptance, independent closure review, SANDBOX/LIVE; local tests/servers prohibited. Earlier Humaux unavailability is historical; see this relay's read/write receipts.

## Revalidation for repeated FIX2 dispatch (2026-10-31)

- Re-fetched origin/r3/integration and ran the prescribed merge: exit 0, `Already up to date.` The existing local merge e48768ea69e61d7660a3cc08a4cbb3f93c2d9663 remains unpushed; PR head remains d699e74ea6c7c0fa3d4e4683b895b27c86f09136.
- Re-read `gh pr checks 395`, both failed run head SHAs, the Foundation assertion, the plan dispatcher-freeze line, and the required job failure directly from GitHub. Both failing runs are bound to PR head d699e74ea6c7c0fa3d4e4683b895b27c86f09136. The failure table above remains current for that head.
- Foundation artifacts API again returns `total_count: 0`; there is no browser evidence artifact to download. The extractor already sees all 50 declared rows; the route inventory's fixed count and the frozen dispatcher are distinct failures.
- Source diff again shows exactly four literal row insertions. The dispatch/admission additions in handler.go change the closure. The scanner explicitly refuses all rows on pin mismatch before appending them to the resolved inventory. No extractor/allow-list/baseline/pin change or routing workaround was attempted.
- The next valid action belongs to the integrator: independent review of the changed dispatcher closure and an authorized amendment to contracts/route-table-dispatchers.json, followed by the exact 46 + 4 = 50 inventory update and combined CI push. The pin is a reviewed contract, not a generated inventory artifact that this lane may regenerate.
- This relay changes only this existing evidence report. No tests, servers, generators, source changes, commits, pushes, PR comments or background jobs were started. No additional repair attempt was consumed. Historical E3 failure evidence does not establish completion at the local merged head; author diagnosis remains E0, source inspection only.
- Shared Humaux memory/coord/canvas tools remain unavailable; no store, claim or canvas update was performed or claimed. WAIT_RUN: NONE (no new push). GREEN and independent acceptance remain NOT_RUN.

## Current FIX2 handoff — protected-pin conflict confirmed from GitHub (2026-10-31)

This section supersedes the prior relay's tool-availability statement only: Humaux is available in this session; a problem record and this worker's canvas handoff are being written under an acquired coordination lock. Earlier output reports were treated as leads; the following facts were independently read from current source and GitHub.

`git fetch origin r3/integration` and `git merge origin/r3/integration` both exited 0 (`Already up to date.`). Local head remains **e48768ea69e61d7660a3cc08a4cbb3f93c2d9663**; PR head remains **d699e74ea6c7c0fa3d4e4683b895b27c86f09136**. Existing untracked evidence is preserved.

| Failing job | Failing test / assertion | Direct evidence | Class and action |
|---|---|---|---|
| Foundation 41096837295 / 124204663270 | `TestGoRepositoryInventory`: `internal/buyerhttp/routes.go rows=50 want46` at go_extract_test.go:320 | `gh run view 41096837295 --job 124204663270 --log-failed` | (b) PR adds exactly four buyer routes. Authorized count is 46 + 4 = 50, but count-only repair cannot resolve the separate freeze refusal; defer to combined integrator repair. |
| Gates 41096837305 / 124208924912, plan | `ERROR (dispatcher freeze)`: buyerhttp.Routes candidate `aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05` differs from reviewed `bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a`; existing `/v1/buyer/cart` then reports BFF_NO_GO | `gh run view 41096837305 --job 124208924912 --log-failed` | (b) Handler dispatch/admission changes alter the frozen closure. The literal rows already match neighbouring declarations. STOP: protected contract requires independent closure review and integrator amendment. |
| Gates 41096837305 / 124210208735, required | `plan=failure build=skipped ... gate=skipped aggregate=skipped ... deploy=skipped ... receipts=skipped` | `gh run view 41096837305 --job 124210208735 --log-failed` | (b) Downstream failure; no separate assertion to repair. |

Foundation artifacts API returns `total_count: 0`; no browser failure/artifact exists. No (a) signed-frame rewrite applies. Base Gates **41105181453** at **a3dc78e4d5ec72831aaf6a376793f0f6a7a56425** is still `in_progress`; its plan job **124232946357** succeeded. Full base GREEN and class (c) are not claimed.

Source proof: `git diff --numstat origin/r3/integration -- internal/buyerhttp/routes.go` is `4 0`; the four rows already use `Method: http.Method*, Path: "literal", kind: ...`. `handler.go` adds admission and dispatch calls. `go_route_tables.go:132–139` refuses the entire table before appending resolved routes on candidate/pin mismatch. `contracts/route-table-dispatchers.json:35` requires explicit independent approval of the entire changed closure. It is a reviewed contract, not a generated route inventory. Both this file and extractor/allow-list changes are expressly forbidden by this card.

**BLOCKED:** integrator must independently review the closure and own the protected hash/functions/declarations amendment, then include the authorized 46 → 50 inventory count in the combined fix. Reformatting valid literals cannot repair the observed hash mismatch. No product/test/contract edit, partial push, workaround, local test/server, or new repair attempt was made here. R2 remains base 139 + assigned migration 0210 = 140. Browser ownership/depmap regeneration was not rerun after the mandatory STOP; no new source input changed.

Evidence level: historical failed CI is an E3 failure receipt for d699e74e only; diagnosis is E0 and no current-head completion/acceptance is claimed. Current relay writes only this required evidence report, with no out-of-card file edits. No background process was started; the final message explicitly hands the blocking action to the integrator. **WAIT_RUN: NONE** (no push). **NOT_RUN:** implementation GREEN, current-head Go/REAL_PG/browser gates, independent closure review, SANDBOX/LIVE; local tests/servers prohibited.

## Latest dispatched verification — 2026-10-31 (current session)

This section supersedes earlier tool-availability/current-status statements for this session only. Humaux is NOT exposed in the current tools; no memory store, coord claim/lock or canvas operation was performed. Applicable minimal-implementation skill: ponytail. No recursive delegation.

Authority was read directly: current dispatched FIX2 card; AGENTS.md; 架构.md §§0/1/2/26/27; contracts/buyer-routes-v1.md and returns-v1.md Amendment 0210; contracts/route-table-dispatchers.json; scanner source; GitHub logs. Output snapshots were only leads, never current-state authority.

First action on the worktree: status inspection preserved all pre-existing untracked evidence. `git fetch origin r3/integration` and `git merge --no-edit origin/r3/integration` exited 0 (`Already up to date`). Base remains a3dc78e4d5ec72831aaf6a376793f0f6a7a56425, local head e48768ea69e61d7660a3cc08a4cbb3f93c2d9663, remote PR #395 head d699e74ea6c7c0fa3d4e4683b895b27c86f09136.

| Failed job | Failing test / assertion | Direct evidence | Class / result |
|---|---|---|---|
| Foundation 41096837295 / 124204663270 | TestGoRepositoryInventory: `internal/buyerhttp/routes.go rows=50 want46`; `routes=293 unresolved=78 notes=95` | `gh run view 41096837295 --job 124204663270 --log-failed`; appended FIX2-red.log receipt | (b), exact count amendment 46 + 4 = 50 is authorized. Count-only repair cannot fix the separate dispatcher freeze; no partial edit/push. |
| Gates 41096837305 / 124208924912 (plan) | `ERROR (dispatcher freeze) UNRESOLVED GET /v1/buyer/cart`: candidate aff6d24766226f455bea9be08b2de8abad3643a71951ad05ede717c28c127c05, reviewed bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a; existing cart/catalog BFF_NO_GO | `gh run view 41096837305 --job 124208924912 --log-failed`; appended FIX2-red.log receipt | (b), changed ServeHTTP/dispatch closure. Literal declarations already match neighbours: diff +4/-0. Protected-pin amendment requires independent closure review and integrator ownership. STOP. |
| Gates 41096837305 / 124210208735 (required) | `plan=failure build=skipped ... gate=skipped aggregate=skipped ... deploy=skipped ... receipts=skipped` | `gh run view 41096837305 --job 124210208735 --log-failed` | (b), downstream dependency failure; no separate assertion to repair. |

Source proof: routes.go:40–43 already uses the existing literal Method/Path/kind shape; handler.go:335–337 and :533–534 alter frozen admission/dispatch bodies. go_route_tables.go:132–139 rejects the whole table before publishing any row on pin mismatch. The protected contract explicitly requires independent full-closure approval; changing formatting cannot repair that mismatch. No extractor, allow-list, baseline or protected pin was edited; no routing workaround was introduced.

Both failed runs currently expose zero artifacts (GitHub artifacts API); no browser job ran, no signed-frame test replacement (a) or browser download applies. No isolated receipts failure exists. Base Gates 41105181453 was read live again and is still in_progress with empty conclusion: base GREEN/class (c) is not established. R2 source pin is base 139 plus assigned 0210 = 140; no new pin needed.

Static receipt: `git diff --check` exit 0. No source inputs changed; ownership/depmap generation was not rerun after mandatory STOP. No new repair attempt, test/server, commit, push, PR edit, shared-branch merge, auto-merge, status posting or background process. Only this existing prescribed report and its filtered log receipt were appended; all prior evidence is preserved. Runtime reports Codex based on GPT-6; actual serving model variant/effort is not exposed.

BLOCKED handoff: integrator must obtain independent review of the changed dispatcher closure and own the protected pin amendment, then combine the authorized inventory count 46 → 50 with the CI push. No additional approach attempted. Historical failures are E3 only for d699e74e; current diagnosis/author judgment E0, no completion claim. WAIT_RUN: NONE (no push). NOT_RUN: current-head GREEN, Go/REAL_PG/browser acceptance, independent closure review, SANDBOX/LIVE; local tests/servers prohibited; Humaux store/canvas unavailable.
