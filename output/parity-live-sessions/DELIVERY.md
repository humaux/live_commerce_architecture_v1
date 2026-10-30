# PARITY-LIVE-SESSIONS · implementation CI handoff · 2026-10-30

Status: implementation source authored; PR #365 GREEN is PENDING. Author assessment E0. Product TypeScript, spec syntax and packet structure are E1 only. Causal tests-only RED is independently inspectable E3 failure evidence at head `395209e39d97c7a5a84373e114879e86f549416c`, run `41052979614`, job `124055728603`: four new C4 assertions failed (missing 直播準備 / 本場商品 at both widths); 23 existing workspace cases passed. The original Foundation pass belongs to that RED head, not this implementation. No local tests, servers, browser, PG or containers were run. No deployment, merge, auto-merge, status posting or Figma write.

## Current implementation and acceptance boundary

- Bare Studio is a filterable session-card list. A selected scene opens preparation; the title action opens the explicit draft-edit task. Existing draft CAS, recovery, timestamp validation and local media permission fences remain; the MOCK rehearsal labels/panel and permanent split-editor landing are removed from the production landing.
- Preparation, post-live review, next-draft copy, end confirmation and product stock tasks use the existing server projections and original scoped/durable command journal. Inventory uses server warehouse/SKU/version and fixed ±1 commands; no client money calculation. Copy exclusions, payment separation and the Meta video boundary remain explicit. A successful copy preserves current receipt conflicts in memory.
- Claims defaults to preflight; existing source/offer/bundle tools are under a real Advanced disclosure. Keyword edit has preview, save, removal preview, server version CAS and reload checks. The advanced tools keep the frozen prompts and original security assertions.
- Meta list → intro → official authorization → Page/IG choice → confirmation → result → detail → capability checks/disconnect. Official OAuth and all existing permission rules remain. Lost pick/disconnect acknowledgements stop writes until an explicit read-only status query. Task navigation focuses the new heading and returns the section to view. SettingsWizard and console product source are untouched.
- New and reused visible copy is enumerated in [COPY.md](COPY.md), including three-locale templates from all six catalogs. Current source syntax/type/packet receipts and file hashes are recorded in `static-checks.json` and `source-hashes.json`; these are E1, never browser acceptance.

**DEFERRED state:** frames `1336:17336/1336:17337`, copy-conflict summary after a document reload only. The POST copy receipt returns `{conflicts:[{keyword,sku_id,reason}]}`, but existing GET Studio/A1 responses do not return a persisted copy-conflict projection. Exact missing field: a server-owned `copy_conflicts: [{keyword, sku_id, reason}]` read projection for the copied session. The happy path, saved draft, offers and closed-window semantics continue; no conflict list is reconstructed or fabricated in the client. Backend/contract expansion and its separate Go RED are for the integrator, not silently added here.

Current Figma readback: all 45 complete `get_design_context(depth=10)` node objects match the signed RED source exactly, and all 45 screenshots were re-read at native frame resolution. See `current-figma/comparison.json`. Earlier PNGs were 0.75-scale exports; current native PNGs sit in `current-figma/`. No Figma canvas was changed.

Density: the supplied probe's visible-ancestor counting applied to the 23 signed mobile source frames gives median **16 nodes / 243 characters**, max **45 nodes** (`figma-density.json`, includes shell and visible-flag text beyond the viewport). Those source counts exceed the default 12/100/20 guidance. The card's exact signed frames remain the UI authority; this is not a density waiver or a product pass. The CI screenshot helper now records actual viewport text-block/character observations with every parity JSON. Browser density and independent visual disposition remain PENDING. Figma copy lint: N/A to canvas writes (none); no banned list was edited. The optional Impeccable detector was NOT_RUN under the owner's explicit local-check whitelist.

### Existing-case mapping (no cases deleted, no budgets relaxed)

| Existing intent | Signed production path / update | Preserved evidence |
|---|---|---|
| STU04 create/edit/CAS, dirty recovery and authorized local connection | Cards → explicit `#edit`; Create session / Connection authorization / Check local connection seller labels | Original idempotency, stale-version, CSRF, revocation, conceal/reveal, native visibility and media-effect assertions |
| STU05 bare-route fetch loop | Visit/reload the bare route, then click its first card | Four rounds and ≤6 read budget unchanged |
| KC16 source, offers, host prompts, bundles and one-time links | Preflight → real Advanced click; metadata uses explicit edit task | Every original source refusal, parser, permission, frozen-prompt and link assertion |
| Meta connect protocol, six granted scopes, IG and disconnect | Intro/bridge/pick/review/result/detail real clicks; granted scope identifiers remain test IDs with seller labels | Cookie/state/redirect, one POST, exact six scope flags, real provider-boundary and unsubscribe assertions |
| MCH11 health/cooldown/role/store fences | Page detail → feature checks; localized schedule action | All eight provider/capability rows, review-required restrictions, 429 and no extra schedule/store leakage assertions |
| New C4 keyword and inventory actions | Preview/save/reload and removal-preview return; ±1/reload/return to products | Stronger real-click checks; no response values replaced |

Click ledger and real-route screenshot matrix below are authored acceptance requirements, not claims of executed clicks. The existing shared helper attaches actual click outcomes (including failed expectations) in CI; no local browser result exists.


## Assignment and authority

Integrator-dispatched implementation worker; branch `unit/parity-live-sessions`; worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-live-sessions`; base `3ea359f796d36153bb4b8aa7259e4e0c27676b86`. Role: implementation worker; one-card implementation phase. No recursive delegation. Runtime model/effort was not independently exposed by a tool: UNKNOWN, not inferred from a role name.

Scope: owner/integrator card in this conversation, FIGMA-INVENTORY §3 C4 and Appendix A U:live-sessions/U:meta-connect. UI authority: Fleet Bridge read-only get_design_context + get_screenshot on all 45 listed frames, file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`; current readback fingerprints and exact visible text are in [frames.json](frames.json). The bridge returned one connected document, name Untitled, fileKey null, defaultFile matching the assigned file; document identity was not strengthened into an independent file-key verification.

The card's literal `ux-live-sessions` and `ux-meta-connect` worktrees do not exist. Read the available ux-live DELIVERY current v3 section and ux-settings current simple/polish sections as contextual handoffs; neither supplies a current C4 three-language table for these 45 older roots. No old DELIVERY section was promoted over live frames. The test translations remain an independent oracle; the current implementation/reused copy is in COPY.md.

Humaux MCP tools are unavailable in this runtime: search, rejected enumeration, coord state, store, code_index/link and canvas update NOT_RUN. No shared-state write/lock was attempted. This local evidence package does not masquerade as a Humaux store or task claim.

## BOURDIEU · Q1–Q6

These are task-based implementation checks, not research findings. Q1: source, keyword, sellable quantity and payment facts must be named at the point of decision. Q2: novice means limited recent experience preparing a live session; the three-step preparation checklist explains where streaming-device checks actually happen. Q3: expert means recent practice with that same task; list filters and a single primary action should shorten repeated work. Q4: mobile and desktop preserve facts and choices; device, class and education are not expertise proxies. Q5: connection success, permission, feature eligibility and provider success stay separate. Q6: aesthetics remains UNKNOWN until owner judgment; no class or taste ranking is claimed.

## PROJECT UNDERSTANDING

Read AGENTS.md, 架构.md §§0/1/2/9/26/27, contracts/tasks.json relevant T09 DAG, studio-v1.md safe projection, live-planning-v1.md, meta-claims-intake-v1.md Merchant connect R4, meta-connection-health-v1.md and current source DTOs. Rules: Go/PG retain business authority; list/draft/copy/lifecycle use existing server facts; a claim is not a paid order; local capture control is not Meta video start/stop; permission/health cannot be inferred from a connected token; no credentials reach the client. Attribution's existing SessionAttribution provides recap, timeline, aggregate buyers and nullable audience facts. No contract/migration/API/lockfile change is included.

## MOBBIN → EAGLE

Read Eagle local API `/api/item/list?folders=MVQL9YTUBXLVO&limit=20`, and item info for `MVOVHEICXGN1F` (Whop desktop moderation) and `MVP3B16M8K9F2` (Twitch mobile stream manager); all returned success. Existing assets/folder were not changed. Structural principles only: desktop keeps the active task and its context together; mobile separates tasks while retaining the selected context. The fixed signed C4 frames remain authority. No external brand, picture, fabricated product fact or provider capability is copied. No new Mobbin import was needed for this tests-only phase.

## FIGMA OFFICIAL GUIDANCE

Read `ux-live/output/ux-audit-ref/FIGMA-GUIDE.md` (guide source date 2026-10-28; the main checkout path in the card is absent). Apply G1 nested responsive layout and explicit sizing; G2 semantic tokens/locales separate from business facts; G3 existing components/states; G4 every control has an observable result and return path; G5 uncertain outcomes and permissions stay separate; G6 real-route metadata and source/evidence boundaries; G7 inventory before composition. [Auto layout](https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma), [variables](https://help.figma.com/hc/en-us/articles/14506821864087-Overview-of-variables-collections-and-modes), [variants](https://help.figma.com/hc/en-us/articles/360056440594-Create-and-use-variants), [prototyping](https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma).

## Required fact parity · implementation acceptance checklist

|Required desktop fact|Mobile frame/state|Review/summary gate|Current evidence|
|---|---|---|---|
|Session identity, preparation/live/end state, schedule, paid count and amount|5:14691; 19:375/376|Server facts, no synthetic Figma numbers|Source/assertions authored; CI PENDING|
|Actual source and permission; target quantity/match mode; products/keywords/prices/availability|19:375; 52:1385|Preflight names the selected session; no IG missing count becomes zero|Source/assertions authored; CI PENDING|
|Camera/microphone/network checked in the streaming tool, not this app|5:17826|Preparation source, device and product steps|Source/assertions authored; CI PENDING|
|Copy title; products/prices copied; source/claims/links excluded; closed claim window|30:2030; 1336:17337|Copy preview → server-created next draft → reload|Source/assertions authored; CI PENDING|
|Selected product/keyword, current and adjusted sellable stock, version/permission fence|52:1388|+1 then −1 through real UI; reread after reload|Source/assertions authored; CI PENDING|
|Selected session and end effect; Meta video and existing payment facts unchanged|1336:17333; 19:376|Cancel, confirm, persisted ended phase|Source/assertions authored; CI PENDING|
|Current/new default keyword; propagation to existing sessions is separate|1336:17335|Preview and removal controls; server save coverage still to extend in implementation|Source/assertions authored; CI PENDING|
|Recap funnel; collected vs pending orders/net; unattributed count; promoted-period spend; aggregate buyers|1151:9425|Values from existing attribution PG fixture; no client money calculation|Source/assertions authored; CI PENDING|
|Timeline timestamp, comments/claims, orders/net; missing Meta spend/viewers remain —|1151:9427|Causation warning and nullable values|Source/assertions authored; CI PENDING|
|Audience status, views vs people, peak concurrency, total time; no buyer demographic join|1151:9431|Real not_read state and audience command receipt|Source/assertions authored; CI PENDING|
|Selected Page identity, IG attachment, existing Pages retained|50:1356/1359; 1108:7306/7308/7310|Pick → confirmation → command → result → reload|Source/assertions authored; CI PENDING|
|Disconnect target, local impact, remote unsubscribe distinction, unrelated Pages retained|50:1358; 948:6718|Lost ACK → read-only query; exactly one POST|Source/assertions authored; CI PENDING|
|Per-provider feature status, limited/review-required state, recheck only schedules|1108:7312|Eight existing permission assertions remain; no invented probe success|Source/assertions authored; CI PENDING|

No checkout recipient, address, delivery total or payment timing exists in these C4 frames; those fields are N/A, not silently omitted from a checkout state.

## RED gate and legacy mapping

The RED commit retained existing tests; this implementation changes their real navigation/labels as mapped above. No case or security assertion is deleted. Existing gate-adapter assertions, drift baseline and dispatcher exemptions are untouched. Six existing browser specs import the new C4 oracle and retain their original runner registration. The console fixture gets two isolated live scenes, separate from the six existing lanes; the attribution runner enables its existing Studio handler for the real /studio route.

|Legacy element asserted absent|Replacement / preserved intent|
|---|---|
|Always-visible Scene settings / 場次設定 split editor|Session cards and create/edit task; original draft/CAS/dirty-recovery intent retained|
|Merchant-facing MOCK rehearsal complementary panel|Preparation/source checklist and real session controls; provider boundary retained|
|MOCK manual comment heading|Source/product/preflight task; existing claim/parser/security tests remain|
|Inline Token status / 權杖狀態 / 令牌状态 dump in connection list|Explicit Page detail → feature checks; original protocol facts remain covered|
|Legacy Attribution & review heading on the Studio route|Session-specific live recap, timeline and audience tasks|

Evidence-only local setup: `node --check` on all seven added/changed TS files exit 0; `gofmt -w` on the two fixture harnesses exit 0. Standalone C4 oracle typecheck exit 0 after explicitly selecting the installed Node type root. Optional broad spec typecheck is NOT_ACCEPTED: missing workspace `@live-commerce/i18n` resolution and pre-existing `Facts.comments.public_unknown` type omission at live-console.spec.ts:204. No assertion/test or fixture was changed to silence that unrelated pre-existing issue; CI remains the test authority.

Click ledger: C4 helpers attach a per-control JSON row, including failures, to the browser artifact. No click is claimed executed locally. CI screenshots live under `parity/<frame-id>.png` at 1440×900 or 390×844; Figma viewports are recorded separately. Prototype-only banners, seeded names/numbers and demo-only navigation wording are excluded under the owner card's product-copy rule. Shell follows owner option A. Source/copy tables and reference density observations are present; browser density and independent visual comparison remain PENDING. No Figma was changed.

## Frame → CI artifact → signed PNG

|Frame|Test family|Browser artifact|Signed PNG readback|
|---|---|---|---|
|1108:7301|meta-connect.spec.ts|`parity/1108:7301.png`|[Admin Desktop · Meta連接 · detailD](figma/get_screenshot-1108-7301-1793361337474-0.png)|
|1108:7302|meta-connect.spec.ts|`parity/1108:7302.png`|[Admin Desktop · Meta連接 · introD](figma/get_screenshot-1108-7302-1793361338236-0.png)|
|1108:7303|meta-connect.spec.ts|`parity/1108:7303.png`|[Admin Desktop · Meta連接 · bridgeD](figma/get_screenshot-1108-7303-1793361338937-0.png)|
|1108:7304|meta-connect.spec.ts|`parity/1108:7304.png`|[Admin Mobile · Meta連接 · bridgeM](figma/get_screenshot-1108-7304-1793361339480-0.png)|
|1108:7305|meta-connect.spec.ts|`parity/1108:7305.png`|[Admin Desktop · Meta連接 · pickD](figma/get_screenshot-1108-7305-1793361340125-0.png)|
|1108:7306|meta-connect.spec.ts|`parity/1108:7306.png`|[Admin Mobile · Meta連接 · pickM](figma/get_screenshot-1108-7306-1793361340863-0.png)|
|1108:7307|meta-connect.spec.ts|`parity/1108:7307.png`|[Admin Desktop · Meta連接 · confirmD](figma/get_screenshot-1108-7307-1793361341567-0.png)|
|1108:7308|meta-connect.spec.ts|`parity/1108:7308.png`|[Admin Mobile · Meta連接 · confirmM](figma/get_screenshot-1108-7308-1793361342455-0.png)|
|1108:7309|meta-connect.spec.ts|`parity/1108:7309.png`|[Admin Desktop · Meta連接 · resultD](figma/get_screenshot-1108-7309-1793361343397-0.png)|
|1108:7310|meta-connect.spec.ts|`parity/1108:7310.png`|[Admin Mobile · Meta連接 · resultM](figma/get_screenshot-1108-7310-1793361344372-0.png)|
|1108:7311|meta-health-ui.spec.ts|`parity/1108:7311.png`|[Admin Desktop · Meta連接 · healthD](figma/get_screenshot-1108-7311-1793361344950-0.png)|
|1108:7312|meta-health-ui.spec.ts|`parity/1108:7312.png`|[Admin Mobile · Meta連接 · healthM](figma/get_screenshot-1108-7312-1793361345457-0.png)|
|1151:9424|attribution.spec.ts|`parity/1151:9424.png`|[Admin Desktop · 直播復盤 · DaWan Live](figma/get_screenshot-1151-9424-1793361346034-0.png)|
|1151:9425|attribution.spec.ts|`parity/1151:9425.png`|[Admin Mobile · 直播復盤 · DaWan Live](figma/get_screenshot-1151-9425-1793361346740-0.png)|
|1151:9426|attribution.spec.ts|`parity/1151:9426.png`|[Admin Desktop · 直播同一時間軸 · DaWan Live](figma/get_screenshot-1151-9426-1793361347389-0.png)|
|1151:9427|attribution.spec.ts|`parity/1151:9427.png`|[Admin Mobile · 直播同一時間軸 · DaWan Live](figma/get_screenshot-1151-9427-1793361347770-0.png)|
|1151:9430|attribution.spec.ts|`parity/1151:9430.png`|[Admin Desktop · 直播觀眾彙總 · DaWan Live](figma/get_screenshot-1151-9430-1793361348519-0.png)|
|1151:9431|attribution.spec.ts|`parity/1151:9431.png`|[Admin Mobile · 直播觀眾彙總 · DaWan Live](figma/get_screenshot-1151-9431-1793361349365-0.png)|
|1336:17329|live-console.spec.ts|`parity/1336:17329.png`|[Admin Desktop · 調整直播庫存 · LC26](figma/get_screenshot-1336-17329-1793361349760-0.png)|
|1336:17332|live-console.spec.ts|`parity/1336:17332.png`|[Admin Desktop · 結束直播 · LC26](figma/get_screenshot-1336-17332-1793361350170-0.png)|
|1336:17333|live-console.spec.ts|`parity/1336:17333.png`|[Admin Mobile · 結束直播 · LC26](figma/get_screenshot-1336-17333-1793361350340-0.png)|
|1336:17334|claims-ui.spec.ts|`parity/1336:17334.png`|[Admin Desktop · 編輯預設留言口令 · LC26](figma/get_screenshot-1336-17334-1793361351123-0.png)|
|1336:17335|claims-ui.spec.ts|`parity/1336:17335.png`|[Admin Mobile · 編輯預設留言口令 · LC26](figma/get_screenshot-1336-17335-1793361351463-0.png)|
|1336:17336|live-console.spec.ts|`parity/1336:17336.png`|[Admin Desktop · 下一場草稿 · LC26](figma/get_screenshot-1336-17336-1793361351912-0.png)|
|1336:17337|live-console.spec.ts|`parity/1336:17337.png`|[Admin Mobile · 下一場草稿 · LC26](figma/get_screenshot-1336-17337-1793361352084-0.png)|
|19:373|claims-ui.spec.ts|`parity/19:373.png`|[Admin Desktop · 開播前準備 · LC26](figma/get_screenshot-19-373-1793361352843-0.png)|
|19:374|live-console.spec.ts|`parity/19:374.png`|[Admin Desktop · 直播後 · LC26](figma/get_screenshot-19-374-1793361353238-0.png)|
|19:375|claims-ui.spec.ts|`parity/19:375.png`|[Admin Mobile · 開播前準備 · LC26](figma/get_screenshot-19-375-1793361353479-0.png)|
|19:376|live-console.spec.ts|`parity/19:376.png`|[Admin Mobile · 直播後 · LC26](figma/get_screenshot-19-376-1793361353667-0.png)|
|30:1977|live-console.spec.ts|`parity/30:1977.png`|[Admin Desktop · 複製成下一場 · LC26](figma/get_screenshot-30-1977-1793361354350-0.png)|
|30:2030|live-console.spec.ts|`parity/30:2030.png`|[Admin Mobile · 複製成下一場 · LC26](figma/get_screenshot-30-2030-1793361354670-0.png)|
|5:14545|studio-ui.spec.ts|`parity/5:14545.png`|[Admin Desktop · 直播場次 · LC26](figma/get_screenshot-5-14545-1793361355141-0.png)|
|5:14691|studio-ui.spec.ts|`parity/5:14691.png`|[Admin Mobile · 直播場次 · LC26](figma/get_screenshot-5-14691-1793361355295-0.png)|
|5:17701|live-console.spec.ts|`parity/5:17701.png`|[Admin Desktop · 直播準備 · SHOPLINE](figma/get_screenshot-5-17701-1793361355963-0.png)|
|5:17826|live-console.spec.ts|`parity/5:17826.png`|[Admin Mobile · 直播準備 · SHOPLINE](figma/get_screenshot-5-17826-1793361356274-0.png)|
|5:17908|meta-connect.spec.ts|`parity/5:17908.png`|[Admin Desktop · Facebook 與 Instagram · SHOPLINE](figma/get_screenshot-5-17908-1793361356939-0.png)|
|50:1356|meta-connect.spec.ts|`parity/50:1356.png`|[Admin Mobile · 連接詳情 · task layout](figma/get_screenshot-50-1356-1793361357392-0.png)|
|50:1358|meta-connect.spec.ts|`parity/50:1358.png`|[Admin Mobile · 中斷連接 · task layout](figma/get_screenshot-50-1358-1793361358135-0.png)|
|50:1359|meta-connect.spec.ts|`parity/50:1359.png`|[Admin Mobile · 連接主頁 · task layout](figma/get_screenshot-50-1359-1793361358420-0.png)|
|52:1385|live-console.spec.ts|`parity/52:1385.png`|[Admin Mobile · 本場商品 · LC26](figma/get_screenshot-52-1385-1793361358650-0.png)|
|52:1388|live-console.spec.ts|`parity/52:1388.png`|[Admin Mobile · 調整直播庫存 · LC26](figma/get_screenshot-52-1388-1793361359071-0.png)|
|6:18045|meta-connect.spec.ts|`parity/6:18045.png`|[Admin Mobile · Facebook 與 Instagram · task layout](figma/get_screenshot-6-18045-1793361359731-0.png)|
|948:6716|meta-connect.spec.ts|`parity/948:6716.png`|[Admin Desktop · 中斷主頁連接確認](figma/get_screenshot-948-6716-1793361360194-0.png)|
|948:6717|meta-connect.spec.ts|`parity/948:6717.png`|[Admin Desktop · 中斷結果待確認](figma/get_screenshot-948-6717-1793361360612-0.png)|
|948:6718|meta-connect.spec.ts|`parity/948:6718.png`|[Admin Mobile · 中斷結果待確認](figma/get_screenshot-948-6718-1793361360744-0.png)|

## Out-of-path disclosure and NOT_RUN

Test registration/fixture files: tests/foundation/browser_ads_attribution_test.go, tests/foundation/browser_live_console_test.go. Imported test helper: tests/admin/parity-live-sessions.ts. Generated: output/ci-select-backend-browser/browser-ownership.json. Card evidence: output/parity-live-sessions/**. Product write paths were untouched in RED; the implementation now changes only the C4 components and its new live-sessions module/i18n namespace. Main checkout read-only.

NOT_RUN/PENDING: current implementation GREEN; actual browser clicks/screenshots/persistence and viewport density; all-browser acceptance; independent E4 comparison; current-SHA CI source-header/dependency gates; Humaux/coord/canvas/code indexing. Packet structure and generated depmap are E1 only. Deferred copy-conflict reload projection is described above. No production or LIVE activity authorized or attempted. Integrator is the wake-up owner for the recorded WAIT_RUN jobs; bounded polling only.
