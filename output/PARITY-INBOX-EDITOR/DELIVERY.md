# PARITY-INBOX-EDITOR — RED candidate, implementation pending

This is a tests-only handoff, not completed UI. Author conclusions E0. Node syntax and deterministic ownership generation are E1. RED on assertions, GREEN, real clicks, browser screenshots and independent comparison are PENDING/NOT_RUN until GitHub runs the suites.

Branch `unit/parity-inbox-editor`; base `3ea359f796d36153bb4b8aa7259e4e0c27676b86`; assigned worktree only. Role: dispatched UI implementer. Runtime model/effort exact IDs are not exposed, UNKNOWN. Main checkout read-only. No recursive delegation. No production action, local tests/server or dependency installation.

## Authority and project understanding

Owner's latest implementation card and final integrator ruling govern scope: signed frames are production UI; only list-row message preview moves to INBOX-PREVIEW. All other work remains in this card.

- `AGENTS.md` and `架构.md` §§0/1/2/26/27: distinguish design/model/mock/live; TypeScript strict frontend; scoped worktree, no backend rules in UI, independent acceptance required.
- `架构.md` §10 and `contracts/meta-inbox-v1.md`: server scope/capabilities/window and durable message outcome govern reply eligibility. Preserve UNKNOWN locks and original-intent reconciliation.
- `contracts/storefront-v2.md` §B and SF-04 continuation: whole-document publication, save before preview/publish, source version, receipt identity, 15-minute draft preview; ambiguous result is neither success nor failure.
- Issue #344 item 5 and final comment read directly with `gh issue view 344`: confirmed own DM in the open window permits follow-ups; private reply remains one per comment. This card does not change private-reply backend rules.
- Current inbox authority: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/DELIVERY.md`, **Current polish delta 2026-10-29** plus retained simplify content for unchanged fields.
- Current editor authority: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/DELIVERY.md`, **SIMPLIFY current delivery** and its current three-language table. Historical fix sections are not current UI authority.
- Fleet Bridge read-only `get_design_context` + screenshots read hub `1429:63114`, sent `1429:38642`, review `1429:65762`, preview `1429:65560`. No Figma writes. Bridge reports Untitled/fileKey null; its default file is not misrepresented as an observed connected key. Fresh PNGs and context receipts remain in this task's `figma/` directory.

## BOURDIEU — Q1–Q6

Q1: novice/expert refers to recent inbox/store-editing experience. Q2: keep direct task rows and stable filter slots. Q3: show actual server versions, publication scope and delivery uncertainty. Q4: infer no class or ability from device. Q5: desktop facts and safe actions must also appear on mobile/result states. Q6: aesthetic approval remains UNKNOWN; this card implements signed content and the owner's shell decision rather than choosing an aesthetic.

## MOBBIN → EAGLE

Reuse structural references from the current module handoffs: Quo `MVNUXG8K4FQ39` separates list/thread/customer context; Chime `MVOVLMVQ5ZSCB` uses concise status recovery; Shopify `MVS1D603YAUS5` uses object rows with a necessary state and clear action. No reference images, brands or product facts copied; no new visual direction. Fresh existing Eagle-folder reads use `MVQL9Z23MS9BM` / `MVS1D5YJKEXW7`, saved as `eagle-inbox.json` / `eagle-editor.json`. No import/write. The editor handoff still requests `SaaS custom domain DNS verification pending TLS`; the integrator owns this Mobbin import. This implementation does not invent verification facts while that reference is unavailable.

## Figma official guidance

Read `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-audit-ref/FIGMA-GUIDE.md` (guide dated 2026-10-28; main-checkout copy absent): G1 [auto layout](https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma)/FILL for list rows and responsive content; G2 [variables](https://help.figma.com/hc/en-us/articles/14506821864087-Overview-of-variables-collections-and-modes) for semantic tokens; G3 [components](https://help.figma.com/hc/en-us/articles/360038662654-Guide-to-components-in-Figma)/variants; G4 [prototyping](https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma) requires clear results and return paths. Tests use native Playwright click/fill/select operations and real-route screenshots at 1440×900 / 390×844. Figma prototypes are not proof of real clicks. This is a dated project-guide read, not current online documentation verification. Current phase changes no visual source.

## Required facts and parity acceptance

|Required desktop field/fact|Mobile and summary/review gate|
|---|---|
|Buyer/channel/store unread/assignment and filter order|Same server-shaped fixture responses; real filter/row clicks, store-wide read effect and FILL check remain in existing suite.|
|Last-message list preview|DEFERRED to INBOX-PREVIEW on all list-containing frames. No decrypted body field is invented; implementation must remove the line without an empty slot.|
|DM recipient, original outgoing body, status and reply deadline|Existing inbox tests retained; new send-state frame screenshots and literal copy assertions on both sizes.|
|Queued/blocked/UNKNOWN obligations and original ACK identity|Existing adversarial tests unchanged. New consecutive DM test proves original completion, refresh persistence, a new idempotency key and a fresh pending lock without another inbound.|
|Draft/live versions, save-before-preview/publish|Literal hub copy and actual fixture versions 42/27; draft 43 after save, live 28 after success. No hard-coded Figma sample versions.|
|Profile name/accent and all optional fields|Existing real-click optional group tests retained; new desktop/mobile default state captures.|
|Home order, navigation targets, page slug/body/empty-page fact|Existing edit/save/reload assertions retained; added reorder, page and menu screenshots.|
|Whole-document scope, immediate effect, unchanged prices/stock/domains, active address|Review and all three result states assert retained scope/address/safe next action.|
|Original request/version while result uncertain|Existing recovery tests plus new query/reload assertions require identical key/body and no duplicate publication.|
|Published content and task centre|Receipt-bound content tests retained; new result, content and post-publication hub screenshots; publish remains hidden after refresh.|
|Preview expiry and unchanged live version|Native preview click opens existing token navigation; signed real-route preview state must also be visible and captured. No buyer fixture HTML is claimed as signed storefront acceptance.|

## Three languages

All visible implementation copy must use the current inbox 62-string and editor 117-string tables in the authority DELIVERY files. They remain the single source; this handoff does not copy historical tables. New tests pin main hub copy and controls independently for zh-TW/zh-CN/en; existing all-locale inbox states and editor outcome tests remain. Three-language runtime execution is NOT_RUN pending CI. Implementation must reconcile dictionary entries with those current tables, not translate zh-TW through character conversion.

## Verification and click ledger

- E1: `node --check` on both changed specs exited 0. `derive-browser-ownership.mjs` exited 0; generated artifact committed with tests.
- TypeScript check attempted, exit 254: local `tsc` absent. No install attempted. CI must compile the tests; compiler/fixture failures do not count as RED.
- `tests/admin/inbox-v2-ui.spec.ts`: existing shared click ledger records new successful assertions; afterEach records scenario failures.
- `tests/admin/editor-v2.spec.ts`: existing click ledger attachment retained and appended after each new accepted path. Failed tests are not called PASS.
- RED must be the PR's own CI failing signed assertions. Record raw run/job IDs and first matching assertion in `red.log` before implementation.
- GREEN must be the PR's own CI at final head. No evidence-only commit after GREEN; final receipts belong in a PR comment.
- Independent reviewer must compare every artifact `parity/<frame-id>.png` against the mapped signed PNG. Missing screenshot/mismatch is unresolved, not a pass.

## Scope and remaining work

Changed source: only two existing browser specs. Generated exception: `output/ci-select-backend-browser/browser-ownership.json`. Task-local `output/PARITY-INBOX-EDITOR/` holds receipts/handoff; no application implementation yet. No migration, OpenAPI, contract, lockfile, route dispatcher, drift baseline or gate-adapter assertion modified. The local tsc absence is a tool limitation, not a reason to weaken CI.

Humaux memory/store/canvas/coord tools unavailable: no store/canvas completion claim. Integrator must register this partial handoff. No task server/container/background test is running. Design read receipts are task-local and no uncertain write was replayed.

## Current signed three-language copy tables (required handoff snapshot)

Snapshot for this card; upstream current DELIVERY sections above remain authority. Values and dynamic versions come from server responses.

### inbox current copy

|zh-TW|zh-CN|en|
|---|---|---|
|14:20|14:20|14:20|
|14:32|14:32|14:32|
|14:35|14:35|14:35|
|2026/10/29 14:27 前回覆|请于 2026/10/29 14:27 前回复|Reply by 2026/10/29 14:27|
|DaWan Live|DaWan Live|DaWan Live|
|Instagram 2|Instagram 2|Instagram 2|
|Instagram · 13:55 · 未讀|Instagram · 13:55 · 未读|Instagram · 13:55 · Unread|
|Instagram · 14:32 · 已讀|Instagram · 14:32 · 已读|Instagram · 14:32 · Read|
|Instagram · 店內已讀|Instagram · 店内已读|Instagram · Read by store|
|Messenger · 13:10 · 已讀|Messenger · 13:10 · 已读|Messenger · 13:10 · Read|
|Messenger · 14:20 · 已讀|Messenger · 14:20 · 已读|Messenger · 14:20 · Read|
|Messenger · 14:20 · 未讀|Messenger · 14:20 · 未读|Messenger · 14:20 · Unread|
|Messenger · 店內已讀|Messenger · 店内已读|Messenger · Read by store|
|仍待確認；到 Instagram 查看，勿重送。|尚未确认；请到 Instagram 查看，不要重发。|Still unconfirmed; check Instagram. Do not resend.|
|仍待確認；到 Messenger 查看，勿重送。|尚未确认；请到 Messenger 查看，不要重发。|Still unconfirmed; check Messenger. Do not resend.|
|傳送|发送|Send|
|傳送中；尚未確認送達，勿重送。|发送中；尚未确认送达，不要重发。|Sending; delivery unconfirmed. Do not resend.|
|傳送遭拒；原文保留，可編輯。|发送被拒；原文已保留，可以编辑。|Send rejected; original text saved for editing.|
|全店訊息|全店消息|Store inbox|
|全部 4|全部 4|All 4|
|到設定檢查連接與回覆權限；確認前無法傳送。|请在设置中检查连接和回复权限；确认前无法发送。|Check the connection and reply permissions in Settings before sending.|
|已看到說明，謝謝。|已看到说明，谢谢。|I saw the information, thank you.|
|已超過回覆時間。|已超过可回复时间。|The reply window has ended.|
|已超過回覆時間；等買家再次傳訊息。|已超过可回复时间；请等待买家再次发消息。|The reply window has ended; wait for the buyer to message again.|
|已送出|已发送|Sent|
|已送出，未代表已讀。|已发送，不代表已读。|Sent does not mean read.|
|已鎖定|发送已锁定|Sending locked|
|想了解材質。|想了解材质。|What material is it?|
|想請問這款的尺寸。|想问一下这款的尺寸。|What are the dimensions of this item?|
|我會先核對尺寸，再回覆你。|我先核实尺寸，再回复你。|I will check the dimensions and get back to you.|
|我會先核對材質，再回覆你。|我先核实材质，再回复你。|I will check the material and get back to you.|
|更新傳送狀態|刷新发送状态|Refresh send status|
|未讀 1|未读 1|Unread 1|
|未讀 2|未读 2|Unread 2|
|本店分派 3|店铺分配 3|Assigned in store 3|
|查看傳送狀態|查看发送状态|View send status|
|查看傳送結果|查看发送结果|View send result|
|查看窗口說明|查看回复时限说明|About the reply window|
|查詢狀態|查询状态|Check status|
|檢查連接權限|检查连接权限|Check connection permissions|
|清除草稿|清空草稿|Clear draft|
|無法傳送|无法发送|Cannot send|
|示範店|示例店铺|Example store|
|示範資料|示例数据|Example data|
|編輯原文|编辑原文|Edit original text|
|缺少回覆權限。|缺少回复权限。|Reply permission is missing.|
|草稿：我會先核對尺寸，再回覆你。|草稿：我先核实尺寸，再回复你。|Draft: I will check the dimensions and get back to you.|
|草稿：我會先核對材質，再回覆你。|草稿：我先核实材质，再回复你。|Draft: I will check the material and get back to you.|
|訊息|消息|Inbox|
|請問商品規格？|请问商品的规格？|What are the product specifications?|
|買家 A|买家 A|Buyer A|
|買家 B|买家 B|Buyer B|
|買家 C|买家 C|Buyer C|
|買家 D|买家 D|Buyer D|
|返回對話|返回对话|Back to conversation|
|送達待確認；只查詢，不重送。|送达情况待确认；只查询，不重发。|Delivery unconfirmed; check status without resending.|
|連接待確認；暫不能回覆。|连接待确认；暂时无法回复。|Connection unconfirmed; replies are unavailable.|
|選擇對話|选择对话|Select a conversation|
|顧客資料 · 客服 A 接管／自動已停|客户资料 · 客服 A 接管／自动回复已停|Customer details · Agent A handling / auto-replies paused|
|顧客資料 · 客服 B 接管／自動已停|客户资料 · 客服 B 接管／自动回复已停|Customer details · Agent B handling / auto-replies paused|
|顧客：買家 A · Instagram<br>已手動連結<br>訂單與累計：未提供<br>客服 A 人工接管 · 自動回覆已停|客户：买家 A · Instagram<br>已手动关联<br>订单与累计消费：未提供<br>客服 A 已接管 · 自动回复已停|Customer: Buyer A · Instagram<br>Manually linked<br>Orders and lifetime spend: not provided<br>Agent A handling · Auto-replies paused|
|顧客：買家 B · Messenger<br>已手動連結<br>訂單與累計：未提供<br>客服 B 人工接管 · 自動回覆已停|客户：买家 B · Messenger<br>已手动关联<br>订单与累计消费：未提供<br>客服 B 已接管 · 自动回复已停|Customer: Buyer B · Messenger<br>Manually linked<br>Orders and lifetime spend: not provided<br>Agent B handling · Auto-replies paused|

### editor current copy

|zh-TW|zh-CN|en|
|---|---|---|
|DaWan Live|DaWan Live|DaWan Live|
|示範店|示例店铺|Demo store|
|商家後台|商家后台|Merchant admin|
|網店|网店|Online store|
|示範店 · 示範|示例店铺 · 示例|Demo|
|編輯網店|编辑网店|Edit store|
|預覽與發布前會先儲存|预览和发布前会先保存|Saves first|
|草稿 12 · 線上 8|草稿 12 · 线上 8|Draft 12 · Live 8|
|預覽草稿|预览草稿|Preview|
|發布|发布|Publish|
|資訊頁|信息页|Pages|
|首頁區塊|首页区块|Home|
|導覽選單|导航菜单|Menus|
|店鋪資料|店铺资料|Store details|
|網店網址|网店网址|Domains|
|首頁區塊|首页区块|Home sections|
|儲存草稿|保存草稿|Save|
|取消變更|取消更改|Cancel|
|首頁：商品列表 → 歡迎文字 → 文字|首页：商品列表 → 欢迎文字 → 文字|Home: product grid → welcome → text|
|歡迎文字|欢迎文字|Welcome|
|⋯|⋯|⋯|
|標題|标题|Heading|
|歡迎來到示範店|欢迎来到示例店铺|Welcome to the demo store|
|上移|上移|Move up|
|下移|下移|Move down|
|移除區塊|移除区块|Remove section|
|文字內容|文字内容|Text content|
|在這裡認識我們的商品。|在这里了解我们的商品。|Explore our products here.|
|商品列表|商品列表|Products|
|全部商品|全部商品|All products|
|商品排序 *|商品排序 *|Product order *|
|最新優先|最新优先|Newest first|
|顯示數量 *|显示数量 *|Item limit *|
|8|8|8|
|文字區塊|文字区块|Text|
|新增區塊|添加区块|Add section|
|未儲存|未保存|Unsaved|
|店鋪名稱 *|店铺名称 *|Name *|
|主題色|主题色|Colour *|
|深綠色 · #247965|深绿色 · #247965|#247965|
|圖片與介紹|图片和介绍|Logo & intro|
|一句話介紹|一句话介绍|Tagline|
|標誌|标志|Logo|
|選擇圖片|选择图片|Choose image|
|網站圖示|网站图标|Browser tab icon|
|公告欄|公告栏|Announcement|
|聯絡資料|联系资料|Contact|
|社群連結|社交链接|Social|
|電子郵件|邮箱|Email|
|電話|电话|Phone|
|聯絡地址|联系地址|Contact address|
|LINE 連結|LINE 链接|LINE link|
|Facebook 粉絲專頁網址|Facebook 公共主页网址|Facebook Page URL|
|Instagram 網址|Instagram 网址|Instagram URL|
|運送說明|配送说明|Shipping|
|頁面標題 *|页面标题 *|Title *|
|網址代稱 *|网址别名 *|URL *|
|shipping|shipping|shipping|
|頁面內容|页面内容|Content|
|運送方式與費用，請依結帳頁顯示為準。|配送方式与费用，请以结账页显示为准。|Delivery methods & fees: checkout.|
|移除此頁|删除此页|Remove this page|
|新增資訊頁|添加信息页|New page|
|退換貨說明|退换货说明|Returns|
|returns|returns|returns|
|尚未填寫|尚未填写|Not set|
|頁首導覽 · 2 / 8|页首导航 · 2 / 8|Header 2/8|
|首頁|首页|Home|
|連結名稱（必填）|链接名称（必填）|Link label (required)|
|連結到|链接到|Links to|
|移除連結|删除链接|Remove link|
|新增連結|添加链接|Add link|
|頁尾導覽 · 1 / 12|页尾导航 · 1 / 12|Footer 1/12|
|草稿 13 預覽 · 15 分鐘有效|草稿 13 预览 · 15 分钟有效|Draft 13 preview · Valid for 15 minutes|
|尚未發布 · 買家看到版本 8|尚未发布 · 买家看到版本 8|Unpublished · Buyers see version 8|
|返回網店|返回网店|Back|
|沒有上架商品|暂无上架商品|No published products|
|草稿 13 · 線上 8|草稿 13 · 线上 8|Draft 13 · Live 8|
|發布草稿 13？|发布草稿 13？|Publish draft 13?|
|發布後，買家立即看到新內容|发布后，买家立即看到新内容|Buyers see the new content immediately|
|https://12345678.xgdwm.com|https://12345678.xgdwm.com|https://12345678.xgdwm.com|
|目前網址可使用|当前网址可用|Current address is active|
|發布店鋪資料、導覽、首頁及全部資訊頁|发布店铺资料、导航、首页和全部信息页|Publishes store details, menus, home and every page|
|不更改價格、庫存與網址|不修改价格、库存和网址|Prices, stock and domains stay unchanged|
|示範店 · #247965 · 無公告或聯絡資料|示例店铺 · #247965 · 无公告或联系资料|Demo store · #247965 · No announcement or contacts|
|首頁：歡迎文字 → 商品列表 → 文字|首页：欢迎文字 → 商品列表 → 文字|Home: welcome → product grid → text|
|頁首：首頁、全部商品；頁尾：運送說明|页首：首页、全部商品；页尾：配送说明|Header: Home, All products; footer: Shipping|
|運送說明、退換貨說明|配送说明、退换货说明|Shipping and returns pages|
|最新優先 · 8 件商品|最新优先 · 8 件商品|Newest first · 8 products|
|退換貨說明：頁面內容留白。|退换货说明：页面内容留空。|Returns information: page content is empty.|
|查看內容|查看内容|View details|
|確認發布草稿 13|确认发布草稿 13|Publish draft 13|
|繼續編輯|继续编辑|Continue editing|
|已發布為版本 9|已发布为版本 9|Published as version 9|
|草稿 13 · 10/28 20:30 台北時間|草稿 13 · 10/28 20:30 台北时间|Draft 13 · Oct 28, 20:30 Taipei|
|查看已發布內容|查看已发布内容|View live|
|這次未能發布|本次发布失败|Publication did not complete|
|沒有發布權限，請聯絡店主|没有发布权限，请联系店主|Ask the store owner for publishing access|
|發布結果待確認|发布结果待确认|Publication awaiting confirmation|
|先查詢結果，請勿再次發布|请先查询结果，勿重复发布|Check the result before publishing again|
|本次發布 · 草稿 13|本次发布 · 草稿 13|This publish · Draft 13|
|查詢發布狀態|查询发布状态|Check publication status|
|上次確認：線上 8；目前待查|上次确认：线上 8；当前待查|Last confirmed: Live 8; current result pending|
|系統分配的網址|系统分配的网址|System-assigned address|
|可使用|可用|Active|
|自有網址|自有网址|Custom address|
|shop.example.com|shop.example.com|shop.example.com|
|等待網址驗證|等待网址验证|Domain verification pending|
|驗證完成前，請使用系統網址|验证完成前，请使用系统网址|Use the system address until verified|
|前往設定|前往设置|Open settings|
|線上版本 9 的內容|线上版本 9 的内容|Live version 9 content|
|草稿 13 · 線上 9|草稿 13 · 线上 9|Draft 13 · Live 9|
|查看網址|查看网址|View store|
|示範資料|示例数据|Demo data|
|返回網店|返回网店|Back to online store|
|目前沒有上架商品|目前没有上架商品|No products are currently available|
|運送說明|配送说明|Shipping information|
|運送方式與費用，請依結帳頁顯示為準。|配送方式与费用，请以结账页显示为准。|Available delivery methods and fees are shown at checkout.|

## Frame-to-artifact map

The PR body contains the full 75-row map. Screenshots are NOT_RUN pending browser CI.
