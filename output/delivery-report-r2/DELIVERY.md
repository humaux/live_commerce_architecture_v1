<!-- Purpose: record the scoped PR48 documentation corrections, source provenance and acceptance limits for DELIVERY-REPORT-R2.
Depends on: docs/delivery/DELIVERY-REPORT.md, docs/runbooks/deploy.md, docs/runbooks/merchant-onboarding.md, pinned repository sources and read-only GitHub PR/CI metadata.
Used by: owner, integrator and the independent full-report reviewer; this record does not authorize release or claim runtime acceptance. -->
# DELIVERY-REPORT-R2 delivery

- Branch: `unit/delivery-report-r2`; base: `228a1c877f63b2be0955060d03ed621a05f4bdcf` (`origin/r3/integration`, PR #49 merge).
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/delivery-report-r2`.
- Delivery revision: the PR head containing this record; no self-referential SHA or ancestor is presented as its own CI receipt.
- Write paths: `docs/delivery/DELIVERY-REPORT.md`, `docs/runbooks/deploy.md` (G2 plus owner-authorized round1 B3/B4 and §6.4 items5/6 / §6.5 item3), `docs/runbooks/merchant-onboarding.md` (G2 sibling wording), this file. No product, contract, migration, gate or dependency changes.
- Evidence: **作者自述记录，E1 / source audit**, plus explicitly historical **MOCK CI** receipts below. Integrator reported the full `claude-sonnet-5-5` review **PASS / no P0/P1 at c19bbe00** with five round1 P2s. That verdict belongs to the prior revision; this batch's new-head CI and integrator delta review remain **pending**, not self-marked PASS. The other-lane review file under untracked `output/ext-agents/` is preserved and not submitted as portable acceptance evidence.
- Owner directive: all tests/builds/application runs on GitHub CI only. No local tests, scripts, Node, pnpm, Go, Playwright, servers or PG were run.

## Provenance

| Contributor | Role / model / reasoning | Base / branch / concrete worktree | Boundary |
| --- | --- | --- | --- |
| Codex-2 `/root` | docs author/integration; `gpt-6.1-sol` / `ultra`; top-level config fields read from `~/.codex/config.toml` (model and effort only) | Original base `228a1c877`, round1 starts from reviewed `c19bbe00efe79ecf1c1c3bba7f1596df7c287bdf`; `unit/delivery-report-r2`; `/Volumes/data/live_commerce_architecture_v1/.worktrees/delivery-report-r2` | Sole file writer; original three paths, expanded to four by owner round1 ruling |
| `/root/r2_deploy_sources` | read-only explorer; `gpt-6.1-sol` / `medium`, explicitly assigned in parent dispatch | Same base/branch/worktree, read-only | G2/B6 route chain, platform-site launch, ads allowance; no code/Git writes or runtime |
| `/root/r2_inventory_sources` | read-only explorer; `gpt-6.1-sol` / `medium`, explicitly assigned in parent dispatch | Same base/branch/worktree, read-only | Staff, preview, CAPI, backup and historical PR provenance; no code/Git writes or runtime |
| `/root/r2_round1_source_audit` | read-only explorer; `gpt-6.1-sol` / `medium`, explicitly assigned in parent dispatch | `c19bbe00`; `unit/delivery-report-r2`; `/Volumes/data/live_commerce_architecture_v1/.worktrees/delivery-report-r2`, read-only | Round1 G2 sibling, CAPI, 0089 grants and PAYUNi boundary; no files/Git writes, runtime or recursive delegation |

Delegate model/effort records are dispatch provenance, not independently verified provider telemetry. Sharing this worktree was for **read-only** source audits; neither delegate was a writer or the final independent reviewer. Coordination task: `56689369-23d9-4195-b846-f3c60d962e45`, agent `codex-delivery-report-r2`. Humaux source audits: `03b54f56-f3f4-44d4-a84d-ca4612c054b4` and `97ff242e-af20-4949-a2c7-09c59f0a97b1`.

Post-edit source consistency audit `f66c30b8-3f25-4005-9f41-ee9640d4f8e5` found the stale pre-0089 grant premise, the backup chapter reference and two incomplete source paths; the author verified and corrected all three before the initial commit. Source assistance does not replace independent acceptance. Round1 task: `977db63e-f50d-417b-b4c1-c5b4fd4bf49c`; owner explicitly expanded the write paths above.

## Corrections and source anchors

Sources are checked at base `228a1c877` unless an exact separate SHA is stated. The report retains C1/C2/C3's original execution trees; this revision does not upgrade their evidence.

| Owner item | Before → correction | Verified source |
| --- | --- | --- |
| 1. G2 / B6 | §0 and §6.3 item4 said claim-source unavailable/404 while item7 said closed → all three statements now agree on conditional route mount; B6 remains App Review/Access Tier | `cmd/api/main.go:175-176`, `cmd/api/claims.go:43-55`, `internal/httpapi/handler.go:210`, `claims.go:130`, `claimsource.go:34-47`; `deploy/README.md:358-362`; existing gate definitions `cmd/api/studio_test.go:53`, `tests/foundation/studio_planning_only_test.go:128-139`, `deploy/scripts/smoke.sh:716-729` (not run here) |
| 2. Public platform site | Omitted → inventory and separate owner launch checklist; historical no-DNS observation is not current status | `docs/delivery/units/platform-site.md:24-54,56-67`, `docs/runbooks/deploy.md:166-168`, `apps/admin/lib/company.ts:6-15`; C2 job below proves only PS1/PS2/PS4, not public deployment or PS3 |
| 3. Ads access / allowance | Permission grant without zero allowance → distinguish current automatic owner bundle from old provisioning, then separate operator budget setting; default **NT$0** is closed | `migrations/0089_staff_team.sql:193-214,428-432`, `contracts/storefront-v2.md:253`; deploy §6.4 item6 / §6.5 item3 now match the owner backfill, with legacy scripts only after actual missing-permission verification. `contracts/meta-ads-v1.md:47,186-187,566,625`, `cmd/meta-admin/ads.go:18-23,66-67` establish NT$0 independently of the local FOLLOWUPS input |
| 4. Staff/team | Missing inventory → invite/accept/role/revoke, 72h token, owner floor, scope/audit and fake-mail evidence | `contracts/storefront-v2.md:137-145,247-258`, `docs/delivery/units/staff-team.md:1-8`, `migrations/0119_live_console_inbox.sql:65-74`; registry `scripts/dev/test-local.sh:292-300`, `docs/delivery/GATES.md:177`; C2 staff test actually passed |
| 5. Buyer design preview | Unqualified preview capability → admin draft/token evidence separated from actual storefront document/Preview tab **NOT_RUN** | `tests/foundation/browser_store_design_test.go:5-16`, `tests/admin/design.spec.ts:484-488`, `docs/delivery/GATES.md:109`, `contracts/storefront-v2.md:228-231` |
| 6. CAPI | Three tables described as one retention class → `insights_daily/capi_events` retention **vs** `capi_contexts` erasure, still a production blocker | `contracts/meta-ads-v1.md:323-328,571`, `migrations/0080_meta_capi.sql:216-231`, `internal/attribution/sweeper.go:65-66`; deploy §6.4 item5 now preserves that distinction and the production registration blocker |
| 7. Historical commits / backup | Bare SHAs and combined checklist → verified commit ancestry with no invented PR association; separate per-upgrade backup and 30-day restore-drill rows | Historical ancestry/metadata below; `docs/runbooks/deploy.md:114,430`, `deploy/scripts/deploy.sh:270`, `docs/runbooks/backup-restore.md` |
| 8. T3 / order-attempt | T3 “NOT_RUN in CI” and missing redesign → report erratum cites real CI workflow; frozen #50 vs pending OA-1/2/3 and UI-only #32 | `docs/delivery/GATES.md:47-51,264-287`, `.github/workflows/gates.yml:22-29,78-87,107,207-218`, `contracts/order-attempt-v1.md:3,7` and §§1–7; PR/coord snapshot below |

Main-checkout `output/integrator/FOLLOWUPS.md` PR48 lines142-146 is an **untracked local input**, not remote acceptance evidence. Thread origins include `4258730154/4258730159/4258730166` (ads/staff/preview) and `4259245968/4259245984` (G2/platform-site). The report labels all FOLLOWUPS-derived rows and the old-fix index **未核验摘要**; PR/thread references are tracing aids, not verified current closure statuses. No local input was added to the branch. GATES.md remains outside the four-path authorization; its stale T3 text is explicitly flagged, not silently treated as current.

## Historical GitHub CI evidence (read only)

Initial query: 2026-10-13 19:07–19:09 UTC; **round1 refreshed metadata at19:31 UTC and re-read filtered logs at19:32–19:35 UTC**. C2 run **38338136795**, attempt1, event schedule, status completed / conclusion success; head **5a54d41b7e86a72eeec2afbe1f7b0458daaf742b**, not R2's head/base. All three job IDs below were re-queried and still completed / success; refreshing the query does not promote historical runtime proof to this revision.

| Receipt | Actual observation | Class / limitation |
| --- | --- | --- |
| [password-auth job115102886582](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886582) | `--- PASS: TestBrowserStaffTeam (18.63s)`; final log explicitly names staff invite/accept/role/revoke, zh-TW/en desktop/390 and loopback SMTP fake | Historical **MOCK**; no real mailbox or production credential |
| [platform-site job115102886936](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886936) | `PASS PS1/PS2/PS4: 30 page cases, 15 SSR cases, 16 tag cases, 390 real clicks, 12 messaging-copy cases and 30 reloads` | Historical **MOCK**; PS3 edge, actual public DNS/host/legal/Meta launch checks not claimed |
| [design job115102886321](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38338136795/job/115102886321) | Run metadata says SUCCESS; harness/gate scope excludes actual storefront document rendering and real-origin Preview tab | Historical **MOCK** admin proof; buyer preview remains **NOT_RUN** |

## 历史提交与 PR 归属

At 19:08 UTC, GitHub `GET /repos/humaux/live_commerce_architecture_v1/commits/<sha>/pulls` returned **zero associated PRs** for both `58a54a401e5b671284ef464517b1ca7c749811e9` and `dc52c110500dfbf044ef3e2f43c19f7450736b92` (command exit0, empty array). PR search found the old umbrella [#1](https://github.com/humaux/live_commerce_architecture_v1/pull/1), but it is **OPEN into main**, not an r3/integration merge receipt; its listed commits also did not establish these associations. No independent PR number can be supplied truthfully from these queries.

`git show -s` identifies the first as the 2026-10-01 CVS T21-01 restock fix and the second as a two-parent worker-authority-split T21-02/03 merge. Each `git merge-base --is-ancestor` returned **0** for both R1 `0bfe36e5` and R2 `228a1c877`, proving these commits are in those histories. This closes the original review's provenance uncertainty by its permitted ancestry check (`output/delivery-report/independent-review-final-5d549660.md:23`); it does **not** complete T21 independent acceptance.

## 订单尝试状态来源

Read-only GitHub query refreshed 2026-10-13 **19:31 UTC**: [#50](https://github.com/humaux/live_commerce_architecture_v1/pull/50) MERGED at16:54:58 UTC, merge `2d97c2027eefaa7df6553a832ffa38aa9fcd9d93`; repo contract header is **FROZEN / DESIGN / runtime NOT_RUN** despite older draft handoff text. [#49](https://github.com/humaux/live_commerce_architecture_v1/pull/49) MERGED at18:51:40 UTC, merge `228a1c877f63b2be0955060d03ed621a05f4bdcf`. [#32](https://github.com/humaux/live_commerce_architecture_v1/pull/32) OPEN at head `77e08616ac67217111e30dc94904ae2f16bbf161`; its eventual scope is UI-only consuming the new backend, not competing placement authority. The same query refreshed [#46](https://github.com/humaux/live_commerce_architecture_v1/pull/46), [#47](https://github.com/humaux/live_commerce_architecture_v1/pull/47) and [issue45](https://github.com/humaux/live_commerce_architecture_v1/issues/45): all OPEN. These are timestamped observations, not permanent states; re-check before deciding.

Historical read-only coordination snapshot **19:05 UTC**: OA-1 task `02066371-e9a4-4e28-96aa-1f5f7c2a00b3` working (Codex-3); OA-2 `183ed0c2-72e3-495a-b530-a202b138ef4c` submitted; OA-3 `3c0b8c9e-e9c9-43cf-bdc3-25b1a0734839` submitted. These states were not refreshed in round1 and are not current implementation claims. Scope mapping is corroborated by [OA-1 brief at70c9e330](https://github.com/humaux/live_commerce_architecture_v1/blob/70c9e330fc60a733defcfc2992e7dcd6cc403f0e/docs/delivery/units/oa-1-order-attempt-core.md), which assigns complete C2 local prerequisites to OA-1, C3/C4 and copy/reminder/CAS to OA-2, public APIs/legacy cutover/A15 and UI integration to OA-3. All full-functional acceptance remains **NOT_RUN**. Re-query before deciding release scope.

## Commands and actual results

All commands below are Git/file inspection or remote read-only queries, **not local tests**.

| Command | Result |
| --- | --- |
| `git fetch origin` | exit0 before worktree creation; base resolved to228a1c877 |
| `git -c core.hooksPath=/dev/null worktree add .worktrees/delivery-report-r2 -b unit/delivery-report-r2 origin/r3/integration` | exit0, new clean worktree |
| `git ls-remote --heads origin refs/heads/unit/delivery-report-r2` | exit0, no remote branch yet (19:10 UTC); before first commit, fetch again and re-check/merge own remote if present |
| `gh pr view 50`, `gh pr view 32`, `gh pr view 49` with JSON metadata | exit0; merged/open/merged respectively, timestamped above |
| `gh api .../commits/58a54a40/pulls`, same fordc52c110 | exit0, both empty associations; not a guessed PR number |
| Four `git merge-base --is-ancestor <58a54a40 or dc52c110> <0bfe36e5 or HEAD228a1c877>` | each exit0; Git history proof, not a test gate |
| `gh run view 38338136795 --json ...` and `gh run view ... --job <id> --log` (filtered) | exit0; exact C2 evidence above, no local execution |
| Static `rg`/`sed` and `git show` of listed sources | Sources inspected; no check-gates/typecheck or executable acceptance invoked. An initial main-checkout search hit an absent contract/glob and was corrected to the assigned/pinned worktree; it is not evidence of a product failure |

Pre-commit remote inspection at **19:18 UTC** again found no `origin/unit/delivery-report-r2` branch, so there was no own remote to merge. Fetched trunk still resolved to base `228a1c877`; the three-document worktree was preserved. Only Git operations/file edits follow this inspection; no local hook/test acceptance is claimed. Final commit/push/PR operation receipts are reported in the handoff and task canvas, independently of this record's non-self-referential revision.

## PR51 round1 P2 response

| Finding | Source correction / record |
| --- | --- |
| 1. B3/B4 | Separate B3 OPEN-1 owner-decision and B4 removed-code rows. `cmd/api/payuni_notify.go` absent from source inventory; `contracts/payuni-wire-v1.md:155-170` explicitly removes the receiver while retaining hosted/query/capture pending OPEN-1. Deploy §0 now matches; receiver-removal test definition exists, **NOT_RUN here** |
| 2. Sibling staleness | merchant-onboarding §4 now agrees with conditional G2 route mount. Deploy §6.4 items5/6 and §6.5 item3 directly corrected against meta-ads §12 and 0089 creator/owner backfill, preserving owner approval, zero allowance and production CAPI blocking |
| 3. Self-record | Report introductory link now says **作者自述记录**, not independent evidence |
| 4. FOLLOWUPS | Every derived row and old-fix index explicitly marked **未核验摘要**; no local integrator file or unverified closure promoted to acceptance |
| 5. GitHub refresh | PR50/49 merged, PR32/46/47 and issue45 OPEN; exact C2 run/head/attempt and three job IDs re-read, with timestamps and historical-SHA boundary above |

Round1 read-only commands: `git fetch origin` → exit0; `git merge --no-edit origin/unit/delivery-report-r2` → exit0 / already up to date; the six `gh pr/issue view` queries → exit0; `gh run view 38338136795 --json ...` → exit0; filtered `--job 115102886582/115102886936 --log` reads → exit0. Static `rg`/`sed`/Git file inventory only, **no local test or application run**. Commit/push receipts and the exact final head are reported in the handoff/canvas; every commit requires a fresh own-remote fetch/merge.

The round1 read-only explorer's post-edit consistency audit returned no concrete P0/P1/P2 in these four documents. It did not independently re-query GitHub or execute runtime; it is E1 author-source assistance, **not** the integrator's delta acceptance. Merchant-onboarding O5 was also synchronized so it cannot reintroduce the old B4 missing-receiver premise.

## CI gates and NOT_RUN

- Prior `c19bbe00` automatic **Foundation38454060006**: SUCCESS at19:31 UTC; **Gates38454060014**: still running at that query, deploy SKIPPED. Neither is proof for this changed revision. New-head automatic Foundation / Gates required: **pending**, run IDs registered after push; docs-only planning is determined in CI, not a local run.
- Integrator reported full `claude-sonnet-5-5` review PASS/noP0P1 on `c19bbe00`; all five P2s addressed above. **Final delta review on this batch remains pending**. Delegate source audits and this author record do not replace it; no self-authored independent PASS.
- All local tests/builds/typecheck/check-gates/browser/release/PG: **NOT_RUN (owner CI-only)**.
- Buyer-origin design Preview tab/document render, actual Meta App Review/LIVE probe, platform public deploy/DNS/TLS/URLs/legal approval, ads production grants/allowance/CAPI registration, production backup/drill/key rotation, full OA-1/2/3 runtime and #32 UI: **NOT_RUN** in this unit.
- Candidate-wide release-gate remains **NOT_RUN**; historical C1/C2/C3 are not an updated whole-R2 acceptance.
- No production/live keys/money/customer messages/PII were accessed; no tests, services, containers or fixture processes were started. Only this unit's branch push/PR is authorized.
