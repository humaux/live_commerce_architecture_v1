# PARITY-LIVE — implementation handoff

Status: RELAY 4 targeted harness repair prepared. Foundation failed on efcec544; replacement own-head CI and independent visual acceptance PENDING. Historical sections below retain prior receipts.
Task: dispatched by Claude integrator; branch unit/parity-live; worktree parity-live; base df35b338b5cfde5de34bb590e2ba118efc17e4a4. Role: UI implementer. Runtime model/effort metadata is not exposed; no sub-agent or recursive delegation.
Authority: owner card 2026-10-30; repository AGENTS.md; 架构.md §§0/1/2/26/27; contracts/live-console-v1.md §§2.5/3.3/4.2/4.4/6/7.1–7.2; existing closed console, claims and comment DTOs.
Visual authority: ux-live/output/ux-live/DELIVERY.md §v3 OWNER-FEEDBACK REWORK (not its superseded v2 frame index), v3/frames.json and v3/exports.json. Fleet Bridge read current desktop 1429:39502, mobile 1429:39607 and public-reply desktop 1429:42507; no Figma writes.

## Scope and authority discrepancy

The card lists 34 predecessor frames, but its common rule explicitly selects the DELIVERY section marked current. The current v3 section identifies 152 finite prototype roots and says 37 old roots were renamed 已被取代. Tests bind all 152 current IDs, retain required restricted/expired/UNKNOWN/queued safety cases, and do not rebuild superseded instructional pages. Current v3 desktop PNGs are 1440×960; the card explicitly requires real-route desktop artifacts at 1440×900, which these tests use. Mobile is 390×844.
Source DELIVERY SHA-256: 8ca5571ac2d411d1d8e0169e3288e2dc1a0f80cdb08db1e4a3bace5742a34c05.

## RED and verification plan

One tests-only RED commit, then wait for its Gates/Foundation runs. RED must fail on newly asserted signed copy/controls on /[locale]/studio/console, not compilation. Only after confirmed assertion RED: implementation push. No local tests/servers. Static commands: node --check tests/admin/live-console.spec.ts; node scripts/dev/derive-browser-ownership.mjs; node scripts/dev/derive-browser-ownership.mjs --check (all exit 0). Browser/REAL_PG runtime, GREEN, independent review and production: NOT_RUN.
The RED commit preserved the existing safety assertions. Implementation adapts the existing real-click paths to mobile tabs and Advanced; lifecycle assertions now read the console status attribute, and revenue assertions read the Advanced revenue region. All expected server/privacy/CAS outcomes remain in place. Existing real Go/PG tests are preserved; new parity cases use explicitly MOCK existing DTOs with authenticated route access. All new outbound reply requests are intercepted; no provider request. No API/contract/migration/pin/lockfile change.

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

## WAIT_RUN handoff

PR #355; head `898e71a829d7f4bce261eebc95ec5028693f8901`. Foundation run 41050615992, job 124045445870: queued. Gates run 41050616128: queued; browser job not created yet. Assertion RED and GREEN remain NOT_RUN. Integrator relays a new session on completion. This post-push status update and red.log remain local; no extra CI-triggering push.

## RELAY 1 — 2026-10-30

Current head remains `898e71a829d7f4bce261eebc95ec5028693f8901`. `gh pr checks 355` confirms Foundation 41050615992/job124045445870 success and browser Gates 41050616128/job124051353169 failure. Completed job logs were retrieved with the jobs API because `gh run view --log-failed --job` refuses logs while the enclosing run is unfinished. Synthetic-merge receipt SHA: `57cbd1545e0651859bf8366829acbabe16d8f232`. The emitted failure is a 13-minute Go timeout in `TestBrowserLiveConsoleRealChain/workspace`, preceded by canceled/connection-refused loopback proxy requests. No Playwright assertion summary was emitted; the failing test and causal root remain UNKNOWN. This is not accepted as the new signed-copy/control RED.

Artifact 12975464801 (gate-browser-live-console, 6688271 bytes) could not be retrieved: configured-proxy download was terminated after more than two minutes without content; one direct-transport attempt timed out at 60 seconds. Both task-owned download processes ended. No timeout, fixture, assertion, gate, source, exemption or threshold was changed. No local test/server/build or push ran in this relay. Existing task-local Figma read receipts were preserved.

Humaux tools appeared after initial discovery in this relay. Task-specific memory search/grep, rejected representative subset (3/9, coverage 0.3333), development-rule original, code map and own-canvas lookup were read. This is an integrator-dispatched child: no new claim or recursive delegation. Scoped shared-state writes use coord locks; historical unavailability above describes the initial RED session only.

Required continuation: integrator provides the failed browser artifact/diagnostic or resolves the shared harness blocker; establish the new assertions' causal RED before implementation. Original run remains attempt 1 with other jobs pending; browser itself is completed/failed. No new WAIT_RUN was created. All implementation, parity screenshot comparison, clicks/persistence, GREEN, independent review, SANDBOX and LIVE remain NOT_RUN. Author judgment is E0; CI timeout and Foundation receipts are environment observations, not PARITY-LIVE completion.

## RELAY 2 — causal RED and implementation

The completed job's partial artifact was recoverable: 45/90-second bounded downloads yielded complete local ZIP members. Each recovered error-context.md was verified against its ZIP data-descriptor CRC32. Desktop frame 1429:39502 and 1429:39683 failed the new signed heading assertion (exact 留言與私訊, element not found, 10s) at tests/admin/live-console.spec.ts. Mobile 1429:39607/39782/39834 failed waiting for the signed 留言 tab (110s). Thus the assertion RED is established at tests-only SHA 898e71a829d7f4bce261eebc95ec5028693f8901, Gates 41050616128/job124051353169; synthetic merge 57cbd1545e0651859bf8366829acbabe16d8f232. The outer job still ended by timeout and is not a full test summary. Raw recovered entries are byte-preserved .md.gz members in red-diagnostics; decompress with gzip -dc; original-byte hashes are in red.log. Their GitHub-generated trailing whitespace is preserved inside the binary envelope, with no gate exemption. This corrects the earlier UNKNOWN assertion status.

Implementation: three desktop regions; real mobile 直播/留言/買家 tabs; named sticky buyer/product/quantity/stock/limit summary; per-buyer/channel in-memory drafts; quick replies; confirmed/queued/uncertain/blocked/expired states; off/on product and pause/resume ordering through existing scoped APIs. UNKNOWN is never replayed. Boolean pending guards contain no buyer text. Pause uses fresh CAS but preserves the clicked target, and blocks on uncertain outcomes across reload. A13 supplies only derived order-count/unpaid facts to the parent; a registration alone does not establish payment. Withheld order data remains unavailable. Nonempty bounded order projections are labelled recent orders, not a total. Old detailed buyer, stock, lifecycle and revenue controls remain under disclosures, exercised by native clicks in the regression spec.

All required field rows above remain assertions to be executed in CI, not author PASS. A/B are synthetic fixture names. Product/stock/order values come from existing responses. No screenshot is used as runtime UI. The video area remains a session/source representation with the existing verified external-post link and unavailable notice; no playable media or provider connection is asserted. Messenger conversations retain the existing scoped private/unreplied views; no message body absent from the response is fabricated. Exact frame comparison, density and complete per-control runtime acceptance remain NOT_RUN until CI and independent review.

Source/transport: merged origin/r3/integration 3ea359f796d36153bb4b8aa7259e4e0c27676b86 into this branch (merge acdee66ba68bfdb83d550fcb222eecda36fe6f48). Its deploy changes are inherited; no task-authored deploy edit. No gate, timeout, baseline, route-dispatcher, API, contract, migration, dependency or lockfile change. tests/ci/next-startup.test.mjs unchanged. Header/Go dependency map regeneration not needed: no Go import graph change.

Static receipts: TypeScript 7.0.2 tsc --noEmit --incremental false with a task-local path-only resolver config: exit 0. The resolver uses existing read-only dependency links and current worktree package sources; initial missing/mismatched package resolution and one source type error were corrected without dependency installation. node --check both changed test files, browser ownership generation/check and git diff --check: exit 0. No Node/Go/PG/browser test, build, server or container ran locally. SSR harness snapshots gained only the already-required window and paid.count fields; import/require harnesses for every touched component were inspected, with no new runtime module allowlist needed. CI must execute the unchanged safety assertions on the implementation head.

Scope outside the original paths, under the card's in-module pre-approval: components/CommentStream.tsx, CommentReply.tsx and LiveWorkspace.tsx for the actual composition/draft owners; components/BuyerPanel.tsx for an optional fenced A13 callback; tests/admin/live-console-render.test.ts for its complete SSR DTO. Console locale owners are src/features/live/{comment-copy,workspace-copy}.ts. Required generated browser-ownership.json and task evidence are listed in the PR. No other application/backend module was edited. Rollback: revert the implementation commit on unit/parity-live, preserving the original RED commit and receipts. No production rollback/deploy was performed.

Humaux tools appeared after initial discovery. Read prior PARITY-LIVE original, own canvas index, current development-rule original 7bdb220b-0798-4b1d-aff5-16ee94cae75f, focused code map/search and representative rejected subset 3/9 (coverage 0.3333; not a full enumeration). Store/canvas writes use scoped coord locks. Integrator-dispatched child: no independent claim or delegation. Wakeup remains the integrator's fresh-session relay on the registered implementation Gates/Foundation runs; no local watcher or background process is left.

## THREE LANGUAGES — authored UI strings

The table covers both console copy owners, including error, permission, advanced and empty states. Dynamic buyer/product/comment/order values are server facts and are not translated or replaced with design samples. Main-route CI cases exercise all three locales at both widths. Browser language fidelity, click ledger actual outcomes, density, all-frame comparison and independent E4 acceptance are PENDING.

|zh-TW|zh-CN|en|
|---|---|---|
|近期訂單|近期订单|Recent orders|
|已載入 1,000 則留言。請重新整理回到最近留言，再載入較早留言。|已加载 1,000 条评论。请刷新回到最近评论，再加载较早评论。|1,000 comments loaded. Refresh to return to recent comments before loading more.|
|已登記|已登记|Registered|
|本場訂單|本场订单|Buyer orders|
|公開回覆已送出|公开回复已发送|Public reply sent|
|留言與私訊|评论与私信|Comments and messages|
|全部|全部|All|
|關鍵字|关键词|Keywords|
|私訊|私信|Private messages|
|待回覆|待回复|Awaiting reply|
|較早的留言|较早的评论|Earlier comments|
|重新整理|刷新|Refresh|
|載入留言中…|正在加载评论…|Loading comments…|
|尚無留言|暂无评论|No comments yet|
|未提供姓名|未提供姓名|Unnamed buyer|
|含附件|含附件|Has attachment|
|查看買家|查看买家|View buyer|
|選擇已關聯認領的留言，查看買家資料。|选择已关联认领的评论，查看买家资料。|Select a comment with a claim to view the buyer.|
|尚無關聯認領，不推測買家身分。|暂无关联认领，不推测买家身份。|No linked claim. Buyer identity is not inferred.|
|私訊|私信|Private message|
|公開回覆|公开回复|Public reply|
|回覆內容|回复内容|Reply text|
|已發佈範本|已发布模板|Published template|
|自行輸入|自行输入|Write a reply|
|送出私訊|发送私信|Send private message|
|提交中…|提交中…|Submitting…|
|每則留言只能私密回覆一次；自動認領回覆也會使用這次機會。|每条评论只能私密回复一次；自动认领回复也会使用这次机会。|Each comment permits one private reply. An automatic claim reply also uses this opportunity.|
|買家主動傳訊後才開啟 24 小時私訊視窗；傳送此私密回覆不會開啟視窗。|买家主动发消息后才开启 24 小时私信窗口；发送此私密回复不会开启窗口。|A buyer's inbound message opens a 24-hour DM window. Sending this private reply does not open it.|
|公開回覆不得包含付款連結。|公开回复不得包含付款链接。|Public replies must not contain payment links.|
|傳送結果未確認。請先到 Messenger 核對；系統不會自動重送。|发送结果未确认。请先到 Messenger 核对；系统不会自动重发。|Delivery is uncertain. Verify in Messenger before taking any further action; this reply will not be resent automatically.|
|已到 Messenger 核對傳送結果|已到 Messenger 核对发送结果|I checked delivery in Messenger|
|已在 Messenger 核對這個場次尚未確認的回覆了嗎？此操作僅解鎖後續回覆，不會重送。|已在 Messenger 核对这个场次尚未确认的回复了吗？此操作仅解锁后续回复，不会重发。|Have you checked this session's pending replies in Messenger? This only unlocks new replies; it does not resend anything.|
|僅測試帳號|仅测试账号|Test accounts only|
|需要回覆訊息權限。|需要回复消息权限。|Reply permission is required.|
|已連接帳號尚未授予這項回覆權限，請到 Facebook 設定重新授權。|已连接账号尚未授予这项回复权限，请到 Facebook 设置重新授权。|The connected account has not granted this reply permission. Reauthorise it in Facebook settings.|
|已連接帳號缺少必要的主頁工作權限，請檢查主頁存取設定。|已连接账号缺少必要的主页工作权限，请检查主页访问设置。|The connected account is missing the required Page task. Review its Page access.|
|已連接帳號尚未訂閱必要事件，請檢查 Facebook 設定。|已连接账号尚未订阅必要事件，请检查 Facebook 设置。|This connected account is not subscribed to the required events. Check Facebook settings.|
|已連接帳號需要到 Facebook 設定重新授權。|已连接账号需要到 Facebook 设置重新授权。|The connected account needs reauthorisation in Facebook settings.|
|平台不支援這項回覆功能。|平台不支持这项回复功能。|The platform does not support this reply action.|
|尚未確認回覆能力，請先檢查帳號連接。|尚未确认回复能力，请先检查账号连接。|Reply capability has not been confirmed. Check the account connection first.|
|系統可能正要自動傳送認領連結。手動私訊會用掉這則留言唯一一次私訊機會，確定繼續？|系统可能正要自动发送认领链接。手动私信会用掉这条评论唯一一次私信机会，确定继续？|The system may be about to send a claim link automatically. A manual private reply uses this comment's only private-reply opportunity. Continue?|
|確認手動回覆|确认手动回复|Confirm manual reply|
|取消|取消|Cancel|
|此買家的認領連結無法送出：這則留言的私訊額度已用。買家回覆私訊後 24 小時內可用一般私訊補發認領連結|此买家的认领链接无法发出：这条评论的私信额度已用。买家回复私信后 24 小时内可用普通私信补发认领链接|This buyer's claim link cannot be sent because this comment's private-reply opportunity is used. After the buyer messages, resend the claim link by DM within 24 hours.|
|留言串已重啟，正在重新載入。|评论流已重启，正在重新加载。|Comment stream restarted. Reloading current comments.|
|頁面隱藏時已清除留言。|页面隐藏时已清除评论。|Comments cleared while this page is hidden.|
|已接受回覆，等待送出結果|已接受回复，等待发送结果|Reply accepted; awaiting delivery|
|私訊已送出|私信已发送|Private message sent|
|傳送失敗|发送失败|Failed|
|已阻擋|已阻止|Blocked|
|確認送出狀態中（不會重複傳）|正在确认发送状态（不会重复发送）|Confirming delivery (will not send again)|
|留言暫時無法載入，請重新整理。|评论暂时无法加载，请刷新。|Comments are temporarily unavailable. Refresh to try again.|
|沒有留言存取權限，請店主確認權限。|没有评论访问权限，请店主确认权限。|You cannot access these comments. Ask a store owner to review your permissions.|
|已限制|已限制|Restricted|
|這位買家已受限制，手動傳送前請確認限制原因。|这位买家已受限制，手动发送前请确认限制原因。|This buyer is restricted. Review the restriction before sending manually.|
|已確認限制，繼續手動回覆|已确认限制，继续手动回复|I have reviewed the restriction|
|已連接帳號目前無法使用這項回覆功能。|已连接账号目前无法使用这项回复功能。|This reply action is unavailable for the connected account.|
|這則留言已使用私密回覆機會。|这条评论已使用私密回复机会。|This comment's private reply has already been used.|
|自動回覆等待中，請勿重複傳送。|自动回复等待中，请勿重复发送。|An automatic reply is pending. Do not send another reply.|
|自動回覆等待中，取代前需要明確確認。|自动回复等待中，替换前需要明确确认。|An automatic reply is pending; explicit confirmation is required to replace it.|
|私密回覆期限已過。|私密回复期限已过。|The private-reply window has expired.|
|Instagram 直播已結束，無法私密回覆。|Instagram 直播已结束，无法私密回复。|The Instagram live session has ended; private reply is unavailable.|
|不支援公開回覆 Instagram 直播留言。|不支持公开回复 Instagram 直播评论。|Public replies to Instagram live comments are not supported.|
|請移除公開回覆中的付款連結。|请移除公开回复中的付款链接。|Remove the payment link from this public reply.|
|近期已有相同回覆，請先核對。|近期已有相同回复，请先核对。|A recent duplicate reply was refused. Check the existing reply.|
|留言資料尚未就緒，回覆前請重新整理。|评论资料尚未就绪，回复前请刷新。|Comment details are not ready. Refresh before replying.|
|無法確認這則留言，請重新整理。|无法确认这条评论，请刷新。|This comment could not be verified. Refresh before replying.|
|無法私密回覆主頁自己的留言。|无法私密回复主页自己的评论。|You cannot send a private reply to a Page's own comment.|
|不支援回覆子留言。|不支持回复子评论。|Replies to nested comments are not supported.|
|請檢查回覆內容及長度。|请检查回复内容及长度。|Check the reply text and its length.|
|請求無效，請重新整理留言。|请求无效，请刷新评论。|This request is invalid. Refresh the comments.|
|留言游標已失效，請重新整理。|评论游标已失效，请刷新。|The comment cursor is no longer valid. Refresh the comments.|
|請先連接貼文或直播來源。|请先连接帖子或直播来源。|Connect a post or live source before reading comments.|
|私訊視窗已關閉。|私信窗口已关闭。|The messaging window is closed.|
|請求暫時受限，請稍後重新整理。|请求暂时受限，请稍后刷新。|Requests are temporarily limited. Wait before refreshing.|
|登入已過期，請重新登入。|登录已过期，请重新登录。|Your session expired. Sign in again.|
|找不到此留言或場次。|找不到此评论或场次。|This comment or session is no longer available.|
|留言串暫時無法使用。|评论流暂时不可用。|The comment stream is temporarily unavailable.|
|無法確認結果，請先核對再操作。|无法确认结果，请先核对再操作。|Unable to confirm the result. Verify before taking further action.|
|無法讀取請求。|无法读取请求。|The request could not be read.|
|請求格式錯誤。|请求格式错误。|The request format is invalid.|
|不支援這項操作。|不支持这项操作。|This action is not supported.|
|直播|直播|Live|
|留言|评论|Comments|
|買家|买家|Buyer|
|結帳連結|结账链接|Checkout link|
|庫存說明|库存说明|Stock details|
|等待買家回覆|等待买家回复|Waiting for the buyer to reply|
|正在確認，請稍候|正在确认，请稍候|Confirming, please wait|
|送出回覆|发送回复|Send reply|
|已登記（還沒付款）|已登记（尚未付款）|Registered (not paid yet)|
|原留言|原评论|Original comment|
|剩餘|剩余|Remaining|
|已售|已售|Sold|
|上限|上限|Limit|
|已保留|已保留|Reserved|
|對話記錄|对话记录|Conversation|
|回覆給|回复给|Reply to|
|商品|商品|Product|
|件|件|items|
|選擇買家，查看留言並回覆。|选择买家，查看评论并回复。|Choose a buyer to read their message and reply.|
|留言先登記，結帳時才留貨。|评论先登记，结账时才预留库存。|Comments register interest. Stock is reserved at checkout.|
|已為你記下{product} {quantity} 件，請點結帳連結確認商品與付款。|已为你登记{product} {quantity} 件，请点结账链接确认商品并付款。|I have registered {quantity} {product} for you. Please use the checkout link to confirm the items and pay.|
|{product}剩餘 {stock} 件，結帳時才留貨。|{product}剩余 {stock} 件，结账时才预留库存。|{product}: {stock} remaining. Stock is reserved at checkout.|
|這款已賣完，我再幫你確認補貨時間。|这款已售罄，我再帮你确认补货时间。|This item is sold out. I will check when it will be back in stock.|
|我再幫你確認商品資訊。|我再帮你确认商品信息。|I will check the product details for you.|
|這則留言已私訊過一次，買家回覆後 24 小時內可再聊。|这条评论已私信过一次，买家回复后 24 小时内可以继续沟通。|This comment already received a private reply. Continue within 24 hours after the buyer replies.|
|送出前會再確認是否能私訊；請先核對這則內容。|发送前会再次确认是否可以私信，请先核对这条内容。|Check the buyer and message before sending.|
|買家資料|买家资料|Buyer details|
|進階|高级|Advanced|
|回覆前摘要|回复前摘要|Reply summary|
|示範資料|演示数据|Demo data|
|已有 5 場開放收單，請先關閉其中一場。|已有 5 场开放收单，请先关闭其中一场。|Five sessions already accept comments. Close one before opening another.|
|帳務限制暫時阻止開播，請核對訂閱狀態。|账务限制暂时阻止开播，请核对订阅状态。|Billing restrictions prevent starting this session. Check your subscription.|
|庫存不可低於已保留、已分配與不可售數量，請重新整理並提高目標數量。|库存不可低于已保留、已分配与不可售数量，请刷新并提高目标数量。|Stock cannot fall below reserved, allocated and unavailable quantities. Refresh and enter a higher target.|
|尚有待確認請求。仍要離開場次嗎？離開後仍須核對回執。|仍有待确认请求。仍要离开场次吗？离开后仍须核对回执。|A request is still pending. Leave this session? Its receipt will still require reconciliation.|
|留言接收|评论接收|Comment feed|
|接收中|接收中|Receiving|
|暫時限流|暂时限流|Temporarily rate-limited|
|請重新連接帳號|请重新连接账号|Reconnect the account|
|暫時無法讀取直播控台。請重新整理確認；系統未推測場次狀態。|暂时无法读取直播控台。请刷新确认；系统未推测场次状态。|The live console is not available. Refresh to check again; no session state has been assumed.|
|尚未開始|尚未开始|Not started|
|直播控台|直播控制台|Live console|
|直播場次|直播场次|Live sessions|
|場次設定|场次设置|Session settings|
|選擇場次|选择场次|Choose a session|
|請先選擇場次，再設定商品與留言收單。|请先选择场次，再设置商品与评论收单。|Choose a session to configure its products and comment ordering.|
|尚無場次，請先到直播場次建立。|尚无场次，请先到直播场次创建。|No sessions yet. Create one in Live sessions.|
|開播前準備|开播前准备|Before going live|
|直播中|直播中|Live session|
|直播後|直播后|After the session|
|已封存|已归档|Archived|
|開始直播|开始直播|Start live session|
|結束直播|结束直播|End live session|
|複製成下一場|复制成下一场|Copy to next session|
|複製上一場|复制上一场|Copy previous session|
|此處控制本場留言收單，不會啟動或停止 Facebook、Instagram 的實際直播。|此处控制本场评论收单，不会启动或停止 Facebook、Instagram 的实际直播。|These controls open or close comment ordering. They do not start or stop your Facebook or Instagram broadcast.|
|結束本場留言收單？平台上的直播不會因此停止。|结束本场评论收单？平台上的直播不会因此停止。|End comment ordering for this session? Your platform broadcast will not be stopped.|
|重新整理|刷新|Refresh|
|正在載入場次…|正在加载场次…|Loading session…|
|登入已過期，請重新登入。|登录已过期，请重新登录。|Your session has expired. Sign in again.|
|你沒有執行此操作的權限。|你没有执行此操作的权限。|You do not have permission for this action.|
|場次已變更，已重新讀取最新資料；請確認後再操作。|场次已变更，已重新读取最新数据；请确认后再操作。|The session changed. The latest state has been loaded; review it before trying again.|
|操作未被接受，請核對最新場次狀態後重試。|操作未被接受，请核对最新场次状态后重试。|The action was not accepted. Check the latest session state before trying again.|
|結果尚未確認，其他操作已鎖定。請核對結果或重試同一請求，不要新增請求。|结果尚未确认，其他操作已锁定。请核对结果或重试同一请求，不要新增请求。|The result is not confirmed. Other actions are locked. Check the result or retry this same request; do not create a new request.|
|無法安全還原先前請求，請勿重新提交。保留此場次，以原請求回執核對結果。|无法安全还原先前请求，请勿重新提交。保留此场次，以原请求回执核对结果。|The earlier request cannot be restored safely. Do not submit it again. Keep this session and use the original request receipt to reconcile its result.|
|部分場次控制暫時無法讀取；直播統計仍可查看，請重新整理以核對控制狀態。|部分场次控制暂时无法读取；直播统计仍可查看，请刷新以核对控制状态。|Some session controls are unavailable. Live statistics remain visible; refresh to check those controls again.|
|重試同一請求|重试同一请求|Retry the same request|
|更新於|更新于|Updated|
|台北時間|台北时间|Taipei time|
|畫面顯示時每 5 秒更新|画面显示时每 5 秒更新|Refreshes every 5 seconds while visible|
|留言|评论|Comments|
|關鍵字留言|关键词评论|Keyword comments|
|買家|买家|Buyers|
|訂單|订单|Orders|
|訂單金額|订单金额|Order amount|
|營業額（已收款）|营业额（已收款）|Collected revenue|
|尚無資料|暂无数据|Not available|
|已觀察留言|已观察评论|Comments observed|
|平台留言數|平台评论数|Platform comment count|
|本場商品|本场商品|Session products|
|本場尚未加入商品。|本场尚未加入商品。|No products in this session yet.|
|設定商品與留言來源|设置商品与评论来源|Configure products and comment source|
|可售庫存|可售库存|Sellable stock|
|儲存庫存|保存库存|Save stock|
|不追蹤|不追踪|Untracked|
|請先到庫存頁設定倉庫，再調整數量。|请先到库存页设置仓库，再调整数量。|Set a warehouse in Inventory before adjusting stock.|
|需要直播庫存調整或庫存管理權限。|需要直播库存调整或库存管理权限。|Live-stock adjustment or inventory-management permission is required.|
|需要直播管理權限。|需要直播管理权限。|Session-management permission is required.|
|開放|开放|Open|
|關閉|关闭|Closed|
|推薦|推荐|Recommend|
|推薦中|推荐中|Recommended|
|僅在控台標示推薦商品，不發送公開留言。|仅在控台标示推荐商品，不发送公开评论。|Highlights this product in the console only; no public comment is sent.|
|已售罄|已售罄|Sold out|
|庫存偏低|库存偏低|Low stock|
|認領數量|认领数量|Claimed quantity|
|已付款數量|已付款数量|Paid quantity|
|請輸入非負整數，單次增減不可超過 1,000。|请输入非负整数，单次增减不可超过 1,000。|Enter a whole nonnegative quantity, with a change no larger than 1,000.|
|編輯期間庫存已變更，已保留您的輸入；儲存前請重新確認。目前可售庫存：|编辑期间库存已变更，已保留您的输入；保存前请重新确认。当前可售库存：|Stock changed while you were editing. Your input is kept; review before saving. Current sellable stock:|
|確認最新庫存|确认最新库存|Confirm updated stock|
|Facebook 直播／貼文|Facebook 直播／帖子|Facebook broadcast / post|
|在 Facebook 查看貼文|在 Facebook 查看帖子|Open Facebook post|
|會開啟 Facebook 貼文，能否查看取決於帳號權限與貼文可見性。|会打开 Facebook 帖子，能否查看取决于账号权限与帖子可见性。|Opens the linked post on Facebook. Access depends on your account and the post's visibility.|
|尚未連接可驗證的 Facebook 貼文連結，請核對留言來源設定。|尚未连接可验证的 Facebook 帖子链接，请核对评论来源设置。|No verified Facebook post link is connected. Check the comment source settings.|
|Instagram 直播無法嵌入此處，請在 Instagram 觀看；支援的留言另行接收。|Instagram 直播无法嵌入此处，请在 Instagram 观看；支持的评论另行接收。|Instagram Live cannot be embedded here. Watch in Instagram; supported comments are handled separately.|
|本場已結束，商品僅供查看。可複製成下一場繼續準備。|本场已结束，商品仅供查看。可复制成下一场继续准备。|This session has ended; products are read-only. Copy it to prepare the next session.|
|下一場名稱|下一场名称|Next session name|
|建立下一場|创建下一场|Create next session|
|取消|取消|Cancel|
|已複製場次，但部分商品有衝突；開播前請核對商品設定。|已复制场次，但部分商品有冲突；开播前请核对商品设置。|The session was copied with product conflicts. Check its products before going live.|
|場次成果|场次成果|Session results|
|已收款訂單|已收款订单|Paid orders|
|包含跨場次訂單，不可跨場次直接加總。|包含跨场次订单，不可跨场次直接加总。|Includes orders shared with another session; do not sum across sessions.|
|沙盒收款|沙盒收款|Sandbox collected|
|暫時無法讀取場次成果。|暂时无法读取场次成果。|Session results are not available.|
|開啟直播控台|打开直播控台|Open live console|
|留言來源與收單設定|评论来源与收单设置|Comment source and ordering settings|
|僅供查看|仅供查看|Read-only access|
|正在送出…|正在提交…|Sending…|
|當場直播|本场直播|Current broadcast|
|暫停接單|暂停接单|Pause orders|
|繼續接單|继续接单|Resume orders|
|接單中|接单中|Accepting orders|
|暫停接單中|已暂停接单|Orders paused|
|下架商品|下架商品|Unlist product|
|重新上架|重新上架|List product|
|直播|直播|Live|
|留言|评论|Comments|
|買家|买家|Buyer|
|進階|高级|Advanced|
|剩餘|剩余|Remaining|
|已售|已售|Sold|
|每人最多|每人最多|Limit per buyer|
|已保留|已预留|Reserved|
|買家留言 {keyword} 就能登記購買|买家评论 {keyword} 即可登记购买|Comment {keyword} to register this product|
|留言先登記，結帳時才留貨。|评论先登记，结账时才预留库存。|Comments register interest. Stock is reserved at checkout.|

## IMPLEMENTATION WAIT_RUN — current head

Pushed `a6e9876dce5d0b5231f4ca3b83f9fde5a3ef8e99` to unit/parity-live, PR #355 into r3/integration. Own Foundation41060952171/job124080250278 and Gates41060952253 are queued, exact headSha confirmed by native gh run list. No browser job yet; plan job is not acceptance or WAIT_RUN. GREEN/current-head E3/E4, all-frame screenshots/density comparison and independent review remain PENDING/NOT_RUN. Post-push receipts are local only to avoid restarting the fleet. Integrator relays a fresh session when these runs finish. Original RED head/run/job remain above. No task-owned process remains.

## RELAY 3 — delivery badge regression, targeted fix 1

Foundation41060952171/job124080250278 failed on implementation head a6e9876dce5d0b5231f4ca3b83f9fde5a3ef8e99: 15 existing PR31 sold-out delivery-warning assertions (105 cases, 90 pass, 15 fail). This is not the original tests-only RED. Raw filtered receipt: relay3-foundation.log. Root cause: the signed channel-specific composer labels replaced the generic labels shared by comment-row delivery badges. Fix: keep composer labels and add separate three-locale delivery_* keys consumed by the stream badge. Existing sent-only sold-out success condition, unknown/pending fences and all assertions stay unchanged. No new import or harness dependency.

|Delivery badge zh-TW|zh-CN|en|
|---|---|---|
|已傳送|已发送|Sent|
|排隊中|排队中|Queued|
|傳送失敗|发送失败|Failed|
|已阻擋|已阻止|Blocked|
|結果未確認|结果未确认|Delivery uncertain|

These generic labels preserve the existing regression gate verbatim; the signed composer wording remains channel-specific. Static browser-ownership --check and git diff --check exit0. Node24.15.0 --check failed to parse the pre-existing TypeScript type alias, including the strip-types/stdin variants; no syntax PASS is claimed. No tsc binary or node_modules in this worktree, so typecheck deferred to CI. No local runtime tests/servers. No page/spec/import/Go changes, so browser ownership generation and Go depmap regeneration are unnecessary; ownership check confirms no drift. Only source changes: CommentStream.tsx and console comment-copy.ts. Humaux memory/canvas/coord/code-index tools unavailable: NOT_RUN. Author judgment E0; new-head CI and independent parity acceptance remain PENDING.

## RELAY 3 WAIT_RUN — current head

RELAY3_PUSH head=efcec5444a3c00c7ca1785df3286637d1ba174d4 pr=355
WAIT_RUN Foundation=41063731968 job=124089669565 status=queued
WAIT_RUN Gates=41063731987 status=pending gate_jobs=NOT_CREATED
GREEN=PENDING own_PR_CI; author=E0; no runtime acceptance claimed
logs=gh run view 41063731968 --log-failed --job 124089669565 (once completed)
wakeup=integrator fresh-session relay on these runs; no task-owned process/watcher; bounded CI lookup ended below10min
Post-push receipts remain local; no evidence-only push.


## RELAY 4 — complete the frozen A1 render fixture

Authority: RELAY 4 card; Foundation run41063731968/job124089669565, head efcec5444a3c00c7ca1785df3286637d1ba174d4. Failed step: Node unit suites, tests/admin/live-console-render.test.ts lines52/65; TypeError at Object.values before the existing assertions. Filtered raw receipt: relay4-foundation.log. This is an implementation regression, not a new causal RED.

Root cause: the SSR harness bypasses readConsole/parseConsole and supplied an incomplete A1 snapshot without required capabilities. The signed console now reads those observations for its demo marker. Production parseConsole already rejects missing capability objects; no production fallback or parser weakening is warranted. Complete all required A1 fields in the fixture, bind it with satisfies ConsoleSnapshot, and add zh-TW/zh-CN/en demo-marker assertions for MOCK versus DESIGN/LIVE_READ/LIVE_SEND/absent observations and denied access. Existing degraded-read, poll, IG-notice, permission and refresh assertions remain verbatim. No production source, UI copy, API or contract changes in this relay.

Attempt count: first targeted repair of this missing-capabilities harness blocker. Relay3's generic delivery-copy blocker was different. Both known failed render cases share this fixture; do not weaken either assertion. Relevant component harness search found live-workspace.test.ts (static source inspection) and live-console-render.test.ts (SSR); only the latter executes LiveConsole.

Scope: tests/admin/live-console-render.test.ts is a same-module harness outside the original browser-spec write paths, explicitly listed in PR body for integrator review. Generated browser-ownership.json updated its render-test hash only. Task evidence adds relay4-foundation.log and updates this DELIVERY/red.log. No Go imports changed, so depmap regeneration is unnecessary. Existing untracked Figma read receipts and prior PR-body/comment drafts are preserved and excluded from the commit.

Local commands: node scripts/dev/derive-browser-ownership.mjs exit0; node scripts/dev/derive-browser-ownership.mjs --check exit0; git diff --check exit0. These are static provenance checks, not runtime acceptance. No local tests/server/browser/build, no merge/auto-merge/deploy. New-head GREEN and independent real-route screenshot/click review PENDING. Author judgment E0; Humaux store/canvas/coord/code-index remain NOT_RUN (tools unavailable). Integrator supplies wakeup through the next run-completion relay.
