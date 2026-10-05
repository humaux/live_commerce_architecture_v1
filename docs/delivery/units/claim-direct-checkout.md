# Unit claim-direct-checkout — 认领链接一键直达结帐（不再选规格）

状态：DRAFT（起草人 2026-10-05，待 integrator 审核/冻结）。Base `r3/integration` `4863db0`。
迁移号建议 **0116**（仅售完提示所需的一个只读 definer；**待 integrator 确认**）。
两段：后端小改（DeepSeek，`.worktrees/claim-direct-checkout`，branch `unit/claim-direct-checkout`）→ B1/B2 形状冻结后
storefront UI（Codex，`.worktrees/claim-direct-checkout-ui`）。先读 `docs/delivery/PROCESS.md`、`AGENTS.md`、本文件；
合同只读 `contracts/live-keyword-claims-v1.md` §0.1(f)(g)、§1、§4.4、§6、§7.2、§11.1 storefront 段、「Live tools (R4)」第 4–6 条。

## 目标与 owner 原话

owner 2026-10-05 描述的流程：① 每个 SKU 有关键字；② 买家在直播中留言关键字；③ 系统发出该 SKU 的链接；
④ **买家点开后直接进入付款，SKU 已选好，没有选择步骤**。
现状（`apps/storefront/components/ClaimLink.tsx`）：预览 → 显式「加入購物車」（B2 redeem）→ 页内购物车 → 买家要自己去 `/{locale}/cart`
→ `/{locale}/checkout`（`CheckoutFlow.tsx`）。本单元把「加入購物車」换成一个主按钮「直接結帳」，一次点击 = redeem + 跳到结帐页，
结帐页里认领的 SKU 和数量已在，买家只选配送/地址/付款。

## 关键事实（读代码核实）

- B1 `GET /v1/buyer/claim-link`、B2 `POST /v1/buyer/claim-link/redeem`（`internal/claims/buyer.go` `PreviewLink`:81、`RedeemLink`:173；
  合同 §7.2）。B1 行的 `available` = offer active ∧ SKU/商品 active ∧ 币种一致（`buyer.go:78`、`availableSKUs`:275）——**不看库存**。
  B2 把可用行**合并**进买家现有购物车（§0.1(f)，delta apply R1），跳过的行保持 pending；现有购物车里已有不可售商品 → B2 返回 409（`buyer.go:208`）。
- 服务器**不会在下单后清空购物车**：`apps/storefront/lib/purchase.ts` `continueShopping`（:1008）在买家继续购物时才把购物车置空
  （仅当购物车版本仍等于订单记录的 `cart_version`）。`orderRecoveryRequired`（:755）= 有在途 `checkout`/`next-cart` 日志或已知订单号。
  `writePurchase`（:768）所有购物车/报价写入都在 Web Lock `commerce-purchase-write-v1` 下；**现在的 B2 调用不在这把锁里**。
- 直播价只经 claim origin 生效：`storefront.cart_lines.claim_*` 由 `RedeemLink` 写入；`claims.live_prices` 要求链接 `expires_at > now`
  （Quote 与 Begin 的 `RevalidateQuote` 都查）；链接过期后重报价按目录价（R4 第 5 条，失败即关闭）。
- 库存可售量口径：`Σ(on_hand − reserved − allocated − unavailable)`（`migrations/0086_catalog_v2.sql:200`）；A6 未追踪 SKU
  `catalog.skus.inventory_tracked=false`（0109）永不售完。`commerce_buyer_runtime` 对 `inventory.balances` **没有** SELECT。
- 现有 WebKit 门禁：`scripts/dev/test-local.sh --browser-webkit`（6 步 buyer/order/payment/merchant-buyer/cvs/password-auth，iPhone 15 profile），
  `release-gate.sh` 的 `B-browser-webkit` 行要求 6 步全干净。

## 范围

### 后端（DeepSeek）
1. B1 每行新增 `sold_out: bool` = SKU `inventory_tracked` 且可售量 < 该行 `quantity`（只读、提示性，不锁库存）。
2. B2 `skipped[].reason` 新增 `"sold_out"`：pending 行若售完则跳过、保持 pending（与 `unavailable` 同语义，补货后再点会加入）。
   B2 其余语义不变（合并、delta apply、`expected_bundle_version` CAS、每次点击一个 `Idempotency-Key`）。
3. 新只读 definer `inventory.buyer_sku_availability(p_skus uuid[]) RETURNS TABLE(sku_id uuid, tracked boolean, available bigint)`：
   owner `commerce_inventory_writer`，`search_path=pg_catalog`，scope 取 `app.tenant_id/app.store_id`（买家 GUC 缺失 → 22023），
   ≤50 个 SKU，EXECUTE 仅 `commerce_buyer_runtime`。`availableSKUs` 一次调用取回。不向任何登录角色新增表/列 GRANT。
### UI（Codex，`apps/storefront`）
4. `ClaimLink.tsx`：主按钮「直接結帳 / 直接结账 / Check out now」；一次点击：
   a. 若 `pendingPurchase(context)?.kind === "checkout"`（下单在途）→ **不 redeem**，直接 `location.assign(checkoutPath(locale))` 让结帐页做恢复，
      并提示「你有一筆訂單正在處理，請先完成；之後再點私訊中的連結」。
   b. 若 `knownOrderID(context)` 存在（上一张单已下）或 `next-cart` 日志 → 先 `continueShopping(context, orderID)`（把已下单的旧购物车清空，
      版本不符则 `uncertain`，转结帐页恢复，不继续）。
   c. 在 `navigator.locks.request("commerce-purchase-write-v1", …)` 内调用 B2（`purchase.ts` 新导出 `redeemClaimLink(context, token, expectedBundleVersion)`，
      与 `writePurchase` 同锁、同 `assertNoOrder`）；`Idempotency-Key = crypto.randomUUID()` 每次点击一个，不自动重试。
   d. 成功且「本链接的行已全部在购物车」（`applied` 非空，或 preview 中无 pending 且可用行）→ `location.assign(checkoutPath(locale) + "?from=claim")`。
      token 只在内存，**不放进 URL、存储或 Referer**；跳走即丢。
   e. 全部 pending 行 `sold_out` 或不可用 → 主按钮禁用，显示售完说明，不跳转。部分售完 → 按钮文案「結帳（N 件可買）」，跳过的行列出原因。
   f. 409 → 现有 conflict 视图（「購物車裡有已下架商品…」）+「前往購物車」；401/上下文变化 → 现有 `renew()`（先查 `orderRecoveryRequired`）。
   g. 404（过期/轮换/他人已绑定，字节一致）→ 「連結已失效或已更換，請私訊商家重新取得」，若购物车非空给「查看購物車」。
5. `CheckoutFlow.tsx`：`?from=claim` 时顶部一行说明「直播登記的商品已放入；購物車裡其他商品也會一起結帳，可在下方移除」，读完即
   `history.replaceState` 去掉参数。不改报价/下单/恢复日志逻辑。
6. `claim-contract.ts`：`validClaimPreview` 精确键加 `sold_out`；`validClaimRedeemed` 的 reason 加 `sold_out`。`claim-copy.ts` 三语文案。
   预览里「保留庫存」说明保持（认领不锁货）。

## 已有购物车：合并（推荐）vs 替换

**推荐合并**（保持 §0.1(f) 现语义），理由：
1. 合同、SQL、`RedeemLink` 全不动；替换需要新的服务器写路径（清空 + 写入），等于第二种购物车语义。
2. 试点商家每晚开播，买家跨场认领很常见：合并 = 一张单、一次运费；替换会把上一场尚未付款的认领行（连同直播价 origin）删掉，买家要回头再点旧链接（72 h 内）才能找回，客服成本高。
3. 替换会在买家没同意的情况下删除其购物车内容。
合并的真实风险及处理：① 已下单的旧商品还在购物车里会被二次下单 → 步骤 4b 先 `continueShopping`；② 无关旧商品被一起结帐 → 结帐页提示 + 逐行移除，报价前买家看得到全部行；
③ 旧购物车里有不可售商品导致 B2 409 → 4f。

## 售完 / 过期 / 竞态
- 预览时售完：4e（提示性，不锁）。预览后、Begin 前被别人买走：沿用现有 Quote/`checkout.Begin` 失败路径（`begin_hold` 拒绝，结帐页现有错误文案），不新增保留。
- 链接过期（TTL 固定 72 h，数据库强制，§0.1(g)）：B1/B2 404；已经加入购物车的行 origin 还在，但 Quote/Begin 时 `claims.live_prices` 不再给直播价 → 重报价按目录价，结帐页显示新价格（现有 RevalidateQuote 409 → 重报价）。不延长 TTL。
- 双击/重复点击：按钮 busy 期间禁用；两次独立点击 = 两个键，第二次 B2 为「nothing」，仍跳结帐；不会多出一张单（单只在 Begin 产生）。

## Non-goals（保留）
不存 token（无 local/session storage、无 URL、无缓存）；BeginCheckout 之前不锁库存；不自动 redeem（页面打开不写购物车——防止链接预览/抓取器绑定 bundle，R4 首绑规则）；
不改 Meta 私讯模板（`claim-link/v1`「點此確認你的喊單」仍适用）；不做「一键付款」跳过配送/地址/付款选择；不做独立的「直播购物车」。

## 合同修改（integrator 冻结）
`contracts/live-keyword-claims-v1.md` 新增「Amendment "Direct checkout" (2026-10, migration 0116)」：
1. §7.2 B1 行键加 `sold_out`（定义同上）；B2 `skipped.reason` 加 `sold_out`；§0.1(f) 补「sold_out 与 unavailable 同为跳过且保持 pending」。
2. §11.1 storefront 段：「explicit button → B2」改为「一个主按钮 → B2（同 Web Lock，先处理在途/已下单）→ 跳 `/{locale}/checkout`」；
   409/404 文案按上文；「claims do not reserve stock」说明保留。
3. §3.2/KC03：新 definer `inventory.buyer_sku_availability` 的 EXECUTE 行。
OpenAPI：无文件承载 B1/B2（`core-openapi.json` 只有 8 个 path），§7.2 表为权威；不新建 path。

## 数据 / 迁移 `0116_claim_sold_out_read.sql`（编号待 integrator 确认）
只有上文第 3 条 definer + `COMMENT ON`；无表、无列、无角色。

## 不变量
I01（store 来自 published origin，买家 scope 来自 capability）、I02（每次点击一个幂等键）、I03（不锁不扣，售完只提示）、
I05/I08（价格只由 Quote 决定；直播价仍需未过期链接）、I09（转发的链接绑定转发对象，§1）、I11（token 不入 URL/存储/日志/Referer）、
I16（售完提示不是可关闭的库存控制，真实拒绝仍在 Begin）、I18。

## 写入路径（唯一归属）
后端（DeepSeek V4-Pro）：`internal/claims/buyer.go`（`availableSKUs`、`splitPending`、Preview/Redeem 行字段）、
`migrations/0116_claim_sold_out_read.sql`、`tests/foundation/claim_direct_checkout_test.go`（作者冒烟）。
UI（Codex）：`apps/storefront/components/ClaimLink.tsx`、`apps/storefront/components/CheckoutFlow.tsx`（仅 `?from=claim` 提示）、
`apps/storefront/lib/{purchase.ts（只加 redeemClaimLink）,claim-contract.ts,claim-copy.ts}`、`apps/storefront/tests/claim.test.mjs`。
独立测试（K3）：`tests/foundation/browser_claim_checkout_test.go`（`TestBrowserClaimDirectCheckout`）、`tests/storefront/claim-checkout.mjs`、
`tests/foundation/claim_direct_checkout_gate_test.go`。
integrator：`contracts/live-keyword-claims-v1.md`、`scripts/dev/test-local.sh`（新模式 + webkit 第 7 步）、`scripts/dev/release-gate.sh`（6→7）、`docs/delivery/GATES.md`。

## 执行角色
后端 DeepSeek V4-Pro；`apps/` 只 Codex；独立测试 K3；钱路/安全终审 Claude（直播价、合并下单、token 不外泄）。

## 测试计划（先红后绿）
| ID | 层 | 内容 | 红跑 |
|---|---|---|---|
| CDC01 | REAL_PG | B1 `sold_out`：追踪 SKU 可售 < 数量为 true、A6 未追踪恒 false、跨店 SKU 不可见；B2 跳过 `sold_out` 且行保持 pending，补货后再 redeem 生效 | 现 `availableSKUs` 不看库存 → 售完行被加入 |
| CDC02 | REAL_PG | definer：无买家 GUC 22023、>50 SKU 拒绝、`commerce_runtime`/匿名无 EXECUTE（KC03 行） | — |
| CDC03 | NODE | `claim.test.mjs`：精确键、`redeemClaimLink` 在锁内且 `orderRecoveryRequired` 时拒绝、token 不写入任何 storage/URL（mock `Storage`/`history` 断言） | 去掉锁 → 失败 |
| CDC04 | BROWSER Chromium | 新模式 `--browser-claim-checkout`：MOCK 手动认领 → 商家签发链接 → 买家开 `/zh-TW/claim#t=` → **一次真实点击**「直接結帳」→ 落在 `/zh-TW/checkout`，行 = 认领 SKU×数量、单价 = 直播价 → 选 7-ELEVEN 取货付款下单（无 PSP）；另测：已有购物车他物（合并 + 提示）、上一张单已下（`continueShopping` 后旧商品不在）、在途下单（不 redeem）、部分售完、全部售完（按钮禁用）、过期链接（owner 夹具把 `issued_at/expires_at` 往前推）、过期后结帐按目录价、双击只一个订单、返回键不再显示 token、地址栏/`document.referrer`/存储里无 token；三语；点击台账 | 现 ClaimLink 无跳转 → 必失败 |
| CDC05 | BROWSER WebKit | 同一测试 `LC_BROWSER_ENGINE=webkit`（iPhone 15）作为 `--browser-webkit` 第 7 步 `claim-checkout`；含 16px 输入框断言（防 iOS 聚焦缩放） | — |

## 门禁命令（已核实存在 / 新增已注明）
- 静态：`go build ./... && go vet ./...`、`gofmt -l`、`bash scripts/dev/check-pkgdocs.sh`、`bash scripts/dev/depmap.sh --check`、
  `bash scripts/dev/check-gates.sh`、`bash scripts/dev/test-node.sh`、`python3 scripts/check_packet.py`、storefront typecheck/build。
- PG：`bash scripts/dev/test-focused.sh '^(TestClaimDirectCheckout|TestLiveClaims|TestLiveTools|TestLiveToolsGate)'`。
- 迁移 + checkout 运行路径：`bash scripts/dev/release-gate.sh --strict --only G07`（PROCESS.md §2.6 强制）。
- 浏览器回归（现有）：`bash scripts/dev/test-local.sh --browser-live-claims`、`--browser-order`、`--browser-cvs`、`--browser-e2e`、`--browser-webkit`。
- **新增**：模式 `--browser-claim-checkout`（`TestBrowserClaimDirectCheckout`，env `LC_BROWSER_CLAIM_CHECKOUT_ACCEPTANCE=1`）；
  `--browser-webkit` 加第 7 步 `run_webkit_step claim-checkout LC_BROWSER_CLAIM_CHECKOUT_ACCEPTANCE '^TestBrowserClaimDirectCheckout$' 900s 1`，
  `release-gate.sh` `B-browser-webkit` 由 `wk >= 6` 改 `>= 7`；GATES.md 两行（check-gates 会强制）。均由 integrator 加。

## 证据标签
后端：REAL_PG。浏览器：BROWSER（MOCK 手动认领，取货付款无 PSP）；WebKit = Playwright 引擎，非真机。卡支付一环沿用 SP18 SANDBOX，不在本单元重跑。
Meta 私讯里点开（Messenger/IG 内置浏览器）= NOT_RUN。本文件 = DESIGN。证据 → `/Volumes/data/live_commerce_architecture_v1/output/claim-direct-checkout/`。

## 风险
- R1 合并把无关旧商品带进结帐（已用提示 + 移除缓解；若 owner 要「只结这一场」，需另开「直播专用购物车」单元，不在此硬改为替换）。
- R2 `continueShopping` 在购物车版本已被他处推进时返回 `uncertain`：买家会被带去结帐页恢复，而不是直接付款——安全优先，体验略差。
- R3 Web Locks：老 iOS（<15.4）或部分内置浏览器无 `navigator.locks` → `unavailable`，按钮改为「前往購物車」降级路径；需真机确认。
- R4 售完提示是快照：点击后到 Begin 之间仍可能被买走（Begin 拒绝，现有文案）。
- R5 链接 72 h 内过期导致结帐价从直播价跳回目录价，买家可能投诉；文案需在预览页写清有效期（已有）。

## NOT_RUN / BLOCKED
- NOT_RUN：Messenger / Instagram 内置浏览器真机点击；真机 iOS Safari；卡支付 SANDBOX 下的直达结帐（SP18 另跑）；LIVE 店铺域名。
- BLOCKED：B1/B2 合同修改冻结前 UI 不开工；`--browser-claim-checkout` 模式、webkit 第 7 步、release-gate 6→7 待 integrator；
  合并 vs 替换需 integrator 确认（本稿推荐合并）。

## Integrator 裁决（Claude Opus，2026-10-05）
1. 复制场次**连同直播价一起复制**（对齐 SHOPLINE「复制上一场」，owner 2026-10-05 要求一键复制开播）；R4 live-tools 规则 2「不复制直播价」仅对「从其他场次复制关键字商品」保留，对整场复制作废。
2. 场次归属采用 `claims.live_price_uses ∪ claims.order_origins(0113)`，替换 `claims.order_live_sources`；订单页场次筛选随之改变，须在测试里覆盖「无直播价的认领订单」归入场次。
3. 一单跨多场次：各场全额计入，接口返回 `multi_session_orders` 供界面脚注；不做分摊。
4. A7 `kwc-v1` 否定词/问句词表按草案的保守版本冻结；非 CONTAINS 场次的暂存行可带 `offer_id`（不含留言原文）——接受。
5. MCI02/KC03 的精确权限清单由 integrator 在合并时更新，执行者不改。
6. 已有购物车：**合并**（不替换）；兑换前先 `continueShopping`，已下单的购物车不得二次下单。
7. 迁移号暂定 0114（A5）/0115（A7）/0116（售罄读模型），以合并顺序为准由 integrator 最终分配。
8. 新增门禁模式批准：`--browser-live-flow`、`--browser-claim-checkout`，`--browser-webkit` 增第 7 步 `claim-checkout`（release-gate 行 6→7 步）。
9. 粉专直播中影片挑选保持 MOCK；status 取值与 U1（影片 post_id 是否等于留言 webhook 的 post_id）待 owner 测试账号做 LIVE 探针。
10. 本轮不做店铺级默认匹配模式（复制场次已带模式）。
