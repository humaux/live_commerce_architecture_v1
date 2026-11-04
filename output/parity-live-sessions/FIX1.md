# PARITY-LIVE-SESSIONS-FIX1 · 2026-11-01

Status: **BLOCKED after Relay 2**. The two previously repaired test groups are still RED after directed attempt 2. The current card explicitly forbids a third approach when the same test remains red. See the Relay 2 inventory below. E3 failure evidence only; no implementation acceptance. No local tests, servers, browsers, containers, PG or LIVE operations.

Authority: integrator FIX1 card in this session; AGENTS.md; 架构.md §§0/1/2/26/27; frozen studio-v1 and existing claims/console CAS contracts; signed frame identities in frames.json. Current source is integration `da2cc7a3fc1e6eac11de727b158412ae9aff8ba9` merged as `ee8748534`. Original implementation head: `85bad87abae9eac6c26ad62bed3e7564c340feb0`.

Original tests-only RED: run **41052979614**, job **124055728603**, head `395209e39d97c7a5a84373e114879e86f549416c`: four new C4 missing preparation/products assertions, with 23 existing workspace cases passing (see red.log / original DELIVERY). Directed-fix RED below: run **41082251104**, PR head `85bad87a…`; jobs execute synthetic merge `ed928e1b0f59f474f106bcf1f712d5ef63d6dfd6` in their receipts. This follow-up card explicitly requires one combined fix push, not a new tests-only round.

Base run **41266607808** at `da2cc7a3f` is now verified **success** through `gh run view` on 2026-11-01. The following original inventory was written before that run completed. **No class (c) claim is made without a matching base log line.** The listed failures have direct source/artifact causes. The already merged inbox helper fix is imported only through integration; this card does not edit inbox helpers.

## Every failing job → assertion → original evidence → class

All IDs below belong to [run 41082251104](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41082251104). Paths/line numbers identify the tested source in that run, not shifted current lines. Class a = signed-frame rewrite, b = regression/setup fix, derived = dependent check.

| Job | Test | Failure | Evidence | Class | Resolution |
|---|---|---|---|---|---|
| 124167226776 | ClickSweep shard 1/10 | claims desktop zh-TW: 關鍵字 = 1 件 · 關鍵字+N = N 件 | ledger row r00020: no visible change within 3 s | b | Selected match-mode button is disabled; changing mode still executes the existing CAS path. |
| 124167226988 | ClickSweep shard 10/10 | Studio desktop zh-TW 全部; claims mobile zh-TW quantity mode | r00006 / r00033: no-effect | b | Disable only the currently selected filter/mode; no scanner or exemption change. |
| 124167225945 | ClickSweep shard 2/10 | claims desktop en: Keyword = 1 · keyword+N = N | r00007: no-effect | b | Same shared StudioClaims fix. |
| 124167225738 | ClickSweep shard 3/10 | Studio desktop en: All | r00041: no-effect | b | Same Studio selected-filter fix. |
| 124167225830 | ClickSweep shard 9/10 | Studio mobile zh-TW: 全部 | r00006: no-effect | b | Same Studio selected-filter fix. |
| 124167225625 | LiveConsoleRealChain/workspace | LC-U1 MOCK contract real clicks: en/zh-CN/zh-TW × 1586/390 | live-console.spec.ts:459: live-session-results-<scene> absent | b | Console Sessions link now opens the signed session list, without a scene query that selects a task. |
| 124167225625 | LiveConsoleRealChain/workspace | A5 list-copy continuation of the six LC-U1 cases | Old live-copy-session / inline modal was replaced by signed Copy last session task | a | Use real signed copy-name/review clicks; retain distinct new ID, server title and remaining console copy/CAS/UNKNOWN assertions. |
| 124167225625 | LiveConsoleRealChain/workspace | C4 preparation copy next-draft and post-live: 1440 and 390 | spec.ts:673: 下一場草稿 absent; mock-receipts contains both C4 copy titles | b | useSessionTask reads SPA URL commits as well as native hashchange; copy phase respects explicit #copy. |
| 124167225724 | BrowserStudioUIRealChain (bundle:04) | STU05 bare route settles and first scene opens | studio-ui.spec.ts:443 Live preparation absent; snapshot Loading scenes… | b | navigate restarts its cancelled detail read when key is unchanged; title card opens #preparation. Four rounds and ≤6 GET budget retained, reload assertion added. |
| 124167225918 | BrowserE2EDealLoop (bundle:05) | T12 deal loop | deal-loop.spec.ts:141: waiting for /New scene/; scanner self-test later sees empty precondition | b (shared setup) | createClaimSession helper clicks signed Create session → draft → claims → Advanced; privacy scanner and all payment/claim assertions unchanged. |
| 124167225918 | BrowserLiveTools (bundle:05) | live tools zh-TW/en × desktop/mobile | live-tools.spec.ts:152: waiting for 新增場次 / New scene | b (shared setup) | Same shared createClaimSession helper; original library/import/live-vs-catalog price assertions retained. |
| 124167226383 | BrowserCustomersBilling (bundle:07) | CB11 RESTRICTED refusal then GOOD opens | customers-billing.spec.ts:410: waiting for Open claim window under collapsed Advanced | b (shared setup) | openClaimTools performs real disclosure click; existing 402→200, no other change and closed-window assertions retained; screenshot cases use the same helper. |
| 124197766878 | receipts | Matrix receipts aggregate | Dependent failed browser jobs in the same run | derived | No receipts code or thresholds changed. |
| 124197766799 | aggregate | Required gate aggregate | Depends on failed jobs above | derived | No aggregate code changed. |
| 124201060598 | required | PR required result | Depends on aggregate/receipts above | derived | No status posted; await own CI. |

## Receipt inventory (raw copies remain ignored/local)

Artifact SHA-256 values below are GitHub Actions archive digests from the run artifacts API, not invented checksums of extracted trees. Downloaded logs are independently hashed. No downloaded archive/log/capture is committed.

| Artifact | Artifact id | SHA-256 |
|---|---|---|
| gate-browser-click-sweep_9_10 | 12993176106 | sha256:f9216d762da3a6bdf2058471deaa6d2af30644f02851053c47b8a9d16928f4b2 |
| gate-browser-studio-ui | 12993171753 | sha256:7f364ced7aab04662d428d7e1b6a25f009616fe105d230f10a14037834e85044 |
| gate-browser-click-sweep_3_10 | 12992768179 | sha256:9237580c723e10b2ac1339c636fa31feadb6cf78cad1d9447e8daa9ac0f8d635 |
| gate-browser-click-sweep_10_10 | 12995821682 | sha256:b8832b5223165fba3e323bb6ed00810e231d2042463f0d94247d88f92307114e |
| gate-browser-customers-billing | 12995095624 | sha256:13d1964804225bf4b82b6484b5b0ae3f1c553713bbe84ec57ab7bb524f126cde |
| gate-browser-click-sweep_2_10 | 12994015350 | sha256:bb153fd7258bb1a2a4f473083a681c3de9d89516bf3b6cc18003cd6eab30520a |
| gate-browser-click-sweep_1_10 | 12994014850 | sha256:4fc12ab25adec30f18bc075d1a6784687bdcb876a138e3709150e894ab66553e |
| gate-browser-live-console | 12993865064 | sha256:677f9edc0a3175b57ca8a6e99b3d6996040b270cce42e668eaea03cc11afe381 |
| gate-browser-e2e | 12993792558 | sha256:784f4eb43cc5554997bc7eb3ee02bc3bc43b7859cef833174fb3ae36db3d4e84 |

| Run | Job | Downloaded log SHA-256 |
|---|---|---|
| 41082251104 | 124167225625 | 2dee6556689ed5a7e2b37a1cfca48b165080a1e8ab66cef950467942652ea91a |
| 41082251104 | 124167225724 | 5e17af4a16840329458935179785225e4acc67df68d3292179e44982545bd2b4 |
| 41082251104 | 124167225738 | af30419bbf9b99baa60d228c88e748353620667b5689672dc864069f111283a4 |
| 41082251104 | 124167225830 | 1aa7317383bb39755d20057cce1b8602dfa90ca6f019ba3e02e83f2daa27db9d |
| 41082251104 | 124167225918 | 5320ae51a7a19403064d543633ef1981c0d2c127c064f5c8dcff74632405fb8d |
| 41082251104 | 124167225945 | e5af01cc5eb9410bfa0b64131f44e7cb484774895ce89953aa4df440006e1f6e |
| 41082251104 | 124167226383 | 1b6608f91bc75fccea76d79b35eefe54c5358c5e89ea75de7c6619423040bb35 |
| 41082251104 | 124167226776 | 270d1774e9dd90c957740b17f4cfcf52f9d6a5d840f8223ac80ceb7d4dbdaaf1 |
| 41082251104 | 124167226988 | 6b4bf3e74e800bd72bb69114fdfe267bd4ffcd5bae8cce62eb55089575f9c990 |


## Merge, boundaries and checks

Merge conflict resolutions preserve both C3/C4 attribution groups, both locale exports and the integration read-only settings assertions. Meta connection/health tests now combine the new settings entry with signed detail/feature tasks. No test case was deleted, skipped, timed out later, or given a weaker fixture/threshold. Existing next-startup gate assertions are untouched by this card. Migration/R2/route-dispatcher/contractdrift changes are inherited from integration only.

Additional original-card path disclosures: `apps/admin/components/LiveWorkspace.tsx` (same admin live module, Sessions link root cause); `tests/admin/customers-billing.spec.ts`, `tests/e2e/deal-loop.spec.ts`, `tests/e2e/live-tools.spec.ts` (the failing callers expressly named by FIX1); `output/parity-live-sessions/.gitignore` and `figma-capture-manifest.json` (retention); generated `output/ci-select-backend-browser/browser-ownership.json`. Previously disclosed foundation registration files remain part of the PR. No backend package, migration, contract, OpenAPI or dependency/lockfile edit is added.

The new retention gate rejected 12,650,352 bytes of existing PR output additions. Bulk `figma/` and `current-figma/` captures are removed **from the Git index only**, original local files preserved. `figma-capture-manifest.json` retains file/node identities, capture date/source SHA and each exact hash. Figma version was absent from original receipts and is explicitly UNKNOWN. No new Figma read/write/design is claimed.

Static commands: `node --check` on changed TS specs/helpers (exit 0); `python3 scripts/check_packet.py` (exit 0, packet structure only); `node scripts/dev/derive-browser-ownership.mjs` then `--check` (exit 0); `bash scripts/dev/depmap.sh` (exit 0, generated bytes unchanged). Existing component import allowlists were searched: no new component imports were introduced, so no harness allowance is added. `tsc` using the pre-existing static configuration failed (exit 1) because local Next/Node/CSS type dependencies are absent; no local test was run to compensate. Source hashes below bind these authored static receipts; all functional verification remains CI-only.

NOT_RUN: current-head browser/unit/Go/REAL_PG tests; current-head GREEN; independent review; LIVE/provider/production acceptance. Integrator owns wake-up and fresh-session relay after Gates/Foundation runs finish. If the same assertion is red again, record BLOCKED and escalate rather than a third approach. No deployment, merge, auto-merge, forced push or fabricated status.

## FIX1 source hashes (sha256)

- `apps/admin/components/Studio.tsx`: `e0f296b3f443bbfbe057bcfdfe18111b792b0b17e6efdfc5d04e4140600a23f8`
- `apps/admin/components/StudioClaims.tsx`: `3f3345c9326eec37bd4cbe832bc4cdccf2e72b67cbcc5808dc9e49f1d109c81c`
- `apps/admin/components/LiveWorkspace.tsx`: `b8d45220483145014a78139cd305daf9ff808386bf9dce17e2ed2f9e16bdd3d6`
- `apps/admin/components/live-sessions/SessionTasks.tsx`: `57388a6e5d1e6a4b7669139e20a5ec7b3efa79d8153cec5cfa37c001903f4d95`
- `tests/admin/parity-live-sessions.ts`: `9be45f5c5c315756e07a3e2b67f807eac5ac75d752c19f43a4d0977fc321cc6a`
- `tests/admin/live-console.spec.ts`: `b5aa0e079b35a5c5909bd161525a692ef57db9d69406a2f309b54de3c6794459`
- `tests/admin/studio-ui.spec.ts`: `c53bd85544b1541f3bb259aab084ab0a39a12fa13acd94d1099de48bddee5217`
- `tests/admin/claims-ui.spec.ts`: `21ba06ef60e1db76209ca6a74e10fa907d56ef7dad1c09eebf67238ea1194a53`
- `tests/admin/customers-billing.spec.ts`: `21a107146ba31acd65d0a34d6db2b62cc0d15c2bf71ba735d7fe0f0509ea17b5`
- `tests/e2e/deal-loop.spec.ts`: `070afc2c1ab9d815c1e7e6b5e18abbe7289ad2ec64b93d4168ad05453366de5a`
- `tests/e2e/live-tools.spec.ts`: `71b8ca6cfeb2a186eb6f78f820670c2fd7300fb2a9fbb90ea1e49e5f65adb16c`
- `tests/admin/meta-health-ui.spec.ts`: `83ce3115934a567a6718b1b41fddf6ffbe4b5d14be275c86c68b25d9e89093a7`

## Relay 1: current-head failure inventory and second directed fix

Authoritative latest-head results: PR head `1d77b89cd4775db6fc9a8bc19627e04af8ae27d5`; Foundation run **41267520054**; Gates run **41267520139**, attempt 1, tested synthetic merge `5b30adce3cab5d42a650a464cb9580c54f92e724`. The older **41082251104** is RED evidence for `85bad87a`, not the latest implementation. Read `gh pr checks 365`, `gh run view --job … --log-failed`, then the three failing browser artifacts. Concurrent `gh run view` log cache reads returned `zip: not a valid zip file` for two jobs; direct GitHub jobs/logs API succeeded. This was transport/cache evidence, not a test result.

First required action: fetched and merged `origin/r3/integration`; merge reported **Already up to date** at `da2cc7a3fc1e6eac11de727b158412ae9aff8ba9`. No other branch/worktree changed. A pre-existing dirty ownership hash for live-console.spec.ts was preserved and then regenerated with all source hashes.

| Every current failing job | Failing test / assertion | Raw CI evidence | Class | Second directed fix |
|---|---|---|---|---|
| Foundation 124733309565 (41267520054) | `CI-SPEED-3: generated browser ownership is current and covers the registry` | `tests/ci/backend-coverage.test.mjs:274`; `browser ownership drift: regenerate with node scripts/dev/derive-browser-ownership.mjs`; sole failing Node test | b, generated metadata regression | Run official ownership generator after the last test edit, then `--check`; no hand edits. |
| bundle:05 124734313790 (41267520139) | `TestBrowserE2EDealLoop` / `TestBrowserLiveTools`; deal-loop.spec.ts:141 and live-tools.spec.ts:153 fail in `openClaimTools` | Both playwright.log files: `parity-live-sessions.ts:95`, `locator('summary') resolved to 2 elements` (Advanced and This round); all four live-tools locale/viewport cells fail here | b, shared setup regression | Select `:scope > summary` in the one shared helper, retaining strict uniqueness, real click and visible `#claims-mode` assertion. The scanner failure is downstream of setup; scanner unchanged. |
| bundle:06 124734313755 (41267520139) | `TestBrowserMetaHealthUI`; `reconnect uses the existing OAuth picker and restores the same Page` | meta-health-ui.spec.ts:148: `getByTestId('metaconnect-token')`, expected `data-state=active`, element not found; 12 other cases pass | a, settings entry intentionally replaced by signed collapsed entry | Reopen settings through the existing `enterSettings` real-click helper after reload. Keep the original banner-absence and exact active-state assertions verbatim. OAuth picker/review/result clicks and Page identity checks unchanged. |
| bundle:07 124734313470 (41267520139) | `TestBrowserCustomersBilling`; CB11 RESTRICTED→GOOD case | customers-billing.spec.ts:597 → parity-live-sessions.ts:95: identical two-summary strict violation; 85 pass, 1 fail, 5 did not run | b, same shared setup regression | Same single helper fix; 402→200, closed/open/closed, billing-link, permission and screenshot assertions unchanged. |
| receipts 124740395785 (41267520139) | Every planned mode has one fresh PASS receipt | `matrix-receipts: RED (3 problem(s))`, failed receipts for e2e/meta-health-ui/customers-billing | derived | No receipts changes. |
| required 124740808293 (41267520139) | Required workflow result | gate/receipts failure dependency | derived | No workflow/status changes. |

No class (c) failure is claimed. Known inbox issues are untouched; the current inbox job passed. The current aggregate **124740395685 passed**. All ten click-sweep shards, live-console **124734313604**, and Studio bundle:04 **124734313887** passed on `1d77b89c`; those historical passes do not accept this new head.

This is directed attempt **2** for the shared claim setup blocker. The first attempt replaced obsolete New scene navigation; its helper introduced the nested-summary ambiguity seen above. If the same assertion is red after this push, **BLOCKED**: return its exact CI assertion and do not try a third approach. No fixture, timeout, test case, threshold, exemption, gate-adapter assertion, product code or frozen contract changed in this relay.

Local static commands on these final sources: `node --check tests/admin/parity-live-sessions.ts` (0), `node --check tests/admin/meta-health-ui.spec.ts` (0), `python3 scripts/check_packet.py` (0, packet structure only; incidental timestamp output restored), ownership generation and `--check` (0), `bash scripts/dev/depmap.sh` (0, no generated dependency-map diff), evidence-retention check (0). No local tests or servers. These are E1 only; browser behavior awaits this PR's own CI.

Relay source SHA-256:
- `tests/admin/parity-live-sessions.ts`: `0e6b0e68ae44c283af2db4efef29fbfc573a16cba0c1972f39daee7e049cb0f0`
- `tests/admin/meta-health-ui.spec.ts`: `b1feee2c6e0fb43a395db3a75ba66027cd2ce6f4b0b3b074bd0cadf0768fce42`
- `output/ci-select-backend-browser/browser-ownership.json`: `41a4c5eef692818aafc3700e429e682bb2416a30363e74e84cf231b8e09d5720`

Raw downloaded job logs and browser artifacts stay ignored in `fix1-artifacts/relay1/`; only pointers/hashes are retained here. Downloaded log SHA-256:

- `foundation.log`: `d4cf4a0139a90ee8cd9223e15175acfa22d6c0a6d862166a9fd578865a8d9f17`
- `bundle05.log`: `1a2be31bec2fc9501b5743b08f3aa6cb4a0c7c5d01f8e153faad98e09aa895e6`
- `bundle06.log`: `e2544d5c7ac19d4eee937600a77342f8dea029f7ee54d214629b3fbc3033fe5a`
- `bundle07.log`: `83172f8db794d61c5777af14f9878dc25df21fdb4ef5bf9aa99230560382b1e4`
- `receipts.log`: `4d5d23da1edb57c19bd2c594615a6c0d1aa73070dfaf1a28e6a2f96803ef1a87`

NOT_RUN: final-head CI (pending push/run IDs), all local functional tests (prohibited), independent review/E4, provider SANDBOX/LIVE and production acceptance. Integrator owns wake-up/relay. Humaux/coord/canvas tools are unavailable; no store, claim or canvas update is asserted.

Relay artifact archive digests (GitHub Actions API, run 41267520139):

| Artifact | ID | SHA-256 |
|---|---|---|
| gate-browser-e2e | 13067808862 | abe1535528848e59fe59176e1fec0aff22446632cc747d599103cb47bc7ca365 |
| gate-browser-meta-health-ui | 13068661896 | 4488e1fee07decf5637b9cb92b8e5675d71965e738c6ed53b512a30678991326 |
| gate-browser-customers-billing | 13069050216 | 61fc3cf77ce1186fda0a0f05bbcc895490a41d89826ca0f5af2c3aaef767c371 |

## Relay 2: BLOCKED — directed attempt 2 remains RED

Read on 2026-11-01 from `gh pr checks 365`, `gh run view 41270522783 --job <id> --log-failed`, and the two downloaded browser artifacts. This is the latest PR-head run, not the original card's older run:

- PR head: `16b22886b98bf815163c349425e2672f1993e8f7`.
- Gates: **41270522783**, attempt 1, **failure**, tested synthetic merge `a9fef606eb427242aa5d4c8111ebc619d70915a7`.
- Foundation: **41270522825**, job **124742178049**, **success** on that PR head.
- First required action: fetched and merged integration `fde11c1310fce20b576fc98023bd3003c66c205b`; local merge commit `e42a91e1b`. The merge includes the required Codex co-author trailer. These four directly implicated files are unchanged by that merge: `StudioClaims.tsx`, `parity-live-sessions.ts`, `live-tools.spec.ts`, `customers-billing.spec.ts`.

| Every latest-head failing job | Failing test / exact assertion | Evidence | Class / disposition |
|---|---|---|---|
| bundle:05 **124742946382** | `TestBrowserLiveTools`: all four zh-TW/en × desktop/mobile cases at `live-tools.spec.ts:193:56`; `TimeoutError: locator.selectOption: Timeout 20000ms exceeded`, waiting for `#claims-prompt-keyword`, `element is not visible` | Artifact `gate-browser-e2e` **13068854843**, `live-tools/20261101T035501.628981733/playwright.log:10–143`; all cases pass the old shared-helper assertion and reach the later reload/prompt step. `TestBrowserE2EDealLoop` now passes (its playwright.log:17). | **(a)** Test still assumes the advanced prompt is exposed after reload. Current `StudioClaims.tsx:498` wraps it in the signed Advanced disclosure, opened only for `task=advanced`; the prompt is at lines 539–541. No test rewrite attempted: the same live-tools tests remain RED after attempt 2, triggering this card's stop condition. |
| bundle:07 **124742946232** | `TestBrowserCustomersBilling`, CB11 at `customers-billing.spec.ts:606:23`; `expect(locator).toBeVisible()` fails: billing-restriction alert locator `resolved to 2 elements` | Artifact `gate-browser-customers-billing` **13069431882**, `customers-billing/20261101T035840.351936627/playwright.log:97–119`; matches the Comment source section and Claim window complementary region; 85 pass, 1 fail, 5 do not run. | **(b)** Product renders the same window error twice: `alert("window")` in the new signed section (`StudioClaims.tsx:495`) and advanced rail (`:522`). Alert helper (`:277–280`) also repeats the billing-link test ID. Do not change locator strictness or choose the first match. No product fix attempted: CB11 remains RED after attempt 2, triggering the card's stop condition. |
| receipts **124746757956** | `matrix-receipts: RED (2 problem(s))` | Its raw failed-step log lists only `--browser-customers-billing: status FAIL` and `--browser-e2e: status FAIL`, run 41270522783/1, merge SHA above. | Derived from the two failures above; no receipt/workflow/status edits. |
| required **124746824968** | Required result is failure | PR checks link to run 41270522783; failed gate/receipts dependencies above. | Derived; no workflow/status edits. |

All ten click-sweep jobs, live-console **124742946132**, Studio bundle:04 **124742946050**, Meta-health bundle:06 **124742946084**, aggregate **124746757918**, and Foundation are green **on `16b22886` only**. This is partial E3 evidence, not whole-PR GREEN or acceptance of the unpushed integration merge. No class (c) exception is claimed; the two failures have PR-source causes. Known inbox files remain untouched.

The prior summary strict-mode assertion is fixed; these are later assertions in the **same tests**. The current FIX1 card says: “If the same test is still red after this, do not try a third approach.” This stricter stop condition applies even though the assertions have advanced. **BLOCKED** pending integrator disposition; no third repair, new RED cycle, implementation push, merge to integration, auto-merge, deployment or status posting. The integration merge and this evidence remain local; the PR comment supplies the remotely visible handoff without starting another CI fleet.

| Downloaded artifact | ID | GitHub archive SHA-256 |
|---|---|---|
| gate-browser-e2e | 13068854843 | aa7a940955f43c867e792436845f242623604b785f1c44541e941b08f57aa617 |
| gate-browser-customers-billing | 13069431882 | 2d34dc5c4b8ff8a48dcb5dfda8588e19deaefd4a48b0af2f9be31512fa7a27f2 |

Downloaded `playwright.log` SHA-256: live-tools `73e39213d76d8344f89253f8881ff7d6902011a700a2e80f0544ef3ce7f09c98`; customers-billing `8d635caa04be32632e79a82564cecdb80075f988bf724d622b92ab38cde93905`. Raw captures stay ignored under `fix1-artifacts/relay2/`; none are added to Git. No local test/server/fixture/process was started. No authored source or outside-card file change in Relay 2; imported upstream files belong to the required integration merge. The official ownership generator/check is run after merging, with its result recorded in the handoff.

NOT_RUN: browser/functional tests locally (prohibited), CI on the unpushed local merge/evidence, independent review/E4, provider SANDBOX/LIVE, production acceptance. WAIT_RUN: none; run **41270522783** is completed failure, not a pending run. Humaux/coord/canvas are unavailable; shared store/update cannot be claimed.
