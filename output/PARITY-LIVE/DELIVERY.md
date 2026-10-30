# PARITY-LIVE — tests-only RED handoff

Status: E1 static syntax/ownership only; CI RED PENDING. No implementation or visual acceptance claimed.
Task: dispatched by Claude integrator; branch unit/parity-live; worktree parity-live; base df35b338b5cfde5de34bb590e2ba118efc17e4a4. Role: UI implementer. Runtime model/effort metadata is not exposed; no sub-agent or recursive delegation.
Authority: owner card 2026-10-30; repository AGENTS.md; 架构.md §§0/1/2/26/27; contracts/live-console-v1.md §§2.5/3.3/4.2/4.4/6/7.1–7.2; existing closed console, claims and comment DTOs.
Visual authority: ux-live/output/ux-live/DELIVERY.md §v3 OWNER-FEEDBACK REWORK (not its superseded v2 frame index), v3/frames.json and v3/exports.json. Fleet Bridge read current desktop 1429:39502, mobile 1429:39607 and public-reply desktop 1429:42507; no Figma writes.

## Scope and authority discrepancy

The card lists 34 predecessor frames, but its common rule explicitly selects the DELIVERY section marked current. The current v3 section identifies 152 finite prototype roots and says 37 old roots were renamed 已被取代. Tests bind all 152 current IDs, retain required restricted/expired/UNKNOWN/queued safety cases, and do not rebuild superseded instructional pages. Current v3 desktop PNGs are 1440×960; the card explicitly requires real-route desktop artifacts at 1440×900, which these tests use. Mobile is 390×844.
Source DELIVERY SHA-256: 8ca5571ac2d411d1d8e0169e3288e2dc1a0f80cdb08db1e4a3bace5742a34c05.

## RED and verification plan

One tests-only RED commit, then wait for its Gates/Foundation runs. RED must fail on newly asserted signed copy/controls on /[locale]/studio/console, not compilation. Only after confirmed assertion RED: implementation push. No local tests/servers. Static commands: node --check tests/admin/live-console.spec.ts; node scripts/dev/derive-browser-ownership.mjs; node scripts/dev/derive-browser-ownership.mjs --check (all exit 0). Browser/REAL_PG runtime, GREEN, independent review and production: NOT_RUN.
Every existing safety assertion remains verbatim and ordered; only imports and appended tests were added. Existing real Go/PG tests are preserved; new parity cases use explicitly MOCK existing DTOs with authenticated route access. All new outbound reply requests are intercepted; no provider request. No API/contract/migration/pin/lockfile change.

## DESIGN PARITY — required facts

|Required fact|Desktop and every mobile selected/summary/review state|Gate|
|---|---|---|
|Buyer A/B and original comment|Selected via real buyer control; named sticky reply summary|Exact buyer and 101+2 / ivory question assertions|
|Product, target 2, cap 5, stock 18/0, sold 20|Same sticky summary on all three mobile tabs and desktop|Summary visible/text assertions in each frame case|
|Registration is unpaid|Seller wording preserved on every A summary|Exact 已登記（還沒付款） assertion|
|Separate buyer and public/private drafts|B cannot inherit A; return restores A in current in-memory task|Real fill/click in three locales × two sizes|
|Actual sent receipt|sent/UNKNOWN/public-sent frames reached by real send click to MOCK receipt|One request; state and disabled send assertions|
|Queued is not delivered|queued response does not show 私訊已送出; refresh remains locked|Real click then reload and select again; one request only|
|Restricted, expired and UNKNOWN|Pause/off, refresh, resume/on cannot unlock sends|Real context clicks plus reload and zero reply requests|
|Sold out|Sold-out draft and absent checkout quick reply|Exact sold notice and absent checkout control|
|No developer scaffold or legacy lifecycle list|No MOCK/UNKNOWN/queued visible; old lifecycle phase list absent|Lexical/absence assertions, actual frame screenshot|
|Recipient phone, destination, total, payment timing|N/A: none exists on this console task|No invented checkout facts|

## Click ledger

Each new test attaches click-ledger-outcome with page, test, expected, actual status and result. Frame tests additionally attach the actual prescribed buyer/channel/filter/tab path and screenshot. Three-locale main cases exercise draft fill, both reply channels, buyer switching, quick replies, send, refresh and reselection. Restriction cases exercise pause/resume and off/on with refresh. No evaluate writes, force clicks or dispatchEvent. Only read-only overflow measurement uses evaluate. Outcomes remain NOT_RUN until CI executes; a screenshot alone does not prove a click.

## BOURDIEU

Q1: factual product/registration/stock helpers kept next to the task; comprehension study NOT_RUN. Q2: experienced operators can use direct filters, buyer selection and composer without forced guide; keyboard runtime NOT_RUN. Q3/Q4: no class/device/education proxy used; no new evidence supplied in the card, UNKNOWN. Q5: the three mobile tasks retain the same buyer/reply facts. Q6: aesthetics separate and UNKNOWN until owner; implementation follows approved frames.

## PROJECT UNDERSTANDING

Read architecture §§0/1/2/26/27: distinguish evidence, use existing Go/PG authority, preserve SaaS boundaries and isolated worktree ownership. live-console-v1 §§3.3/4.2: one private reply per comment and 24-hour DM window; §4.4: queued/sent/failed/blocked/unknown remain distinct; §§6/7: capabilities, stock and counts come from existing responses. Read DTOs and direct route/workspace/console/comment/reply consumers. No backend rule is relocated into the browser.

## MOBBIN → EAGLE

No new design or reference is introduced. Implement the already approved module structure. UX-LIVE DELIVERY lists Eagle MVOVHEICXGN1F (Whop desktop) and MVP3B16M8K9F2 (Twitch mobile): desktop stream/tasks and mobile task split are the inherited structural principles. No images, brands or product facts are copied. Fresh Eagle API/folder read and new Mobbin import: NOT_RUN; the module folder ID is not supplied here. No new pattern/query is needed for this implementation card.

## FIGMA OFFICIAL GUIDANCE

The exact preamble path output/ux-audit-ref/FIGMA-GUIDE.md is absent in the read-only main checkout. The module's authoritative DELIVERY cites official auto-layout, variable binding and prototype reaction guides. This card makes no Figma changes: translate its existing horizontal/vertical layout into responsive CSS, preserve component/state boundaries, and verify actual browser controls in CI. Guide revalidation and Figma copy-lint: NOT_RUN (no changed Figma frames).

## THREE LANGUAGES

The current v3 DELIVERY copy table has old→new seller wording, not a three-locale table. Tests pin independent zh-TW, zh-CN and English labels; the implementation handoff must place translations in existing i18n and complete the per-visible-string table. No UI strings have been shipped by this tests-only commit. Main-screen tests cover all three locales and both sizes.

## Ownership, limits and continuation

Outside original write paths: output/ci-select-backend-browser/browser-ownership.json (required generated artifact), output/PARITY-LIVE/DELIVERY.md and red.log (task evidence). Local bridge PNGs are uncommitted read receipts, not production assets. Source changes remain within the live-console browser spec. No background server/process started.
Humaux tools are not available in this runtime: memory_store/canvas/coord/locks NOT_RUN, not represented as successful. Repository evidence is not a substitute for Humaux acceptance. Final E3/E4, independent screenshot review, CI GREEN, implementation and integration remain pending.
