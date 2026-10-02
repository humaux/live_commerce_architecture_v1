# storefront-r5 — UI LOCAL_VERIFIED；G1 DEFERRED

日期：2026-10-02。**G2 / G3 / G4 / G5 / G8 已实现并通过本地验收；等待 Claude 集成终审，不代表生产发布准入。** G1 方案 A 已否决，方案 B 由后端单元承担，本单保留原生 img。

## 工作区与提交

- 唯一写入工作区：`/Volumes/data/live_commerce_architecture_v1/.worktrees/storefront-r5`，分支 `unit/storefront-r5`。
- 原基线：`aca14d7b7794c22950ffffe9f1714117235b2675`；续作起点：`1b7e3a51f0b1847f6a33e3b5dee4ce721f56d0a3`。
- 保留 G1 证据提交：`41e2be869db270a165d41984e260a5d4c417556c`、`1b7e3a51f0b1847f6a33e3b5dee4ce721f56d0a3`。
- `a170583bdc7a92f9b014af8a43996b2955845f8c`：五项 UI、三语文案和单测。
- `069637cbd64c92230800f940bd427b9e5770625a`：统一 NT$ 显示及金额精度测试。
- `a3ccc2fa43577b910118edd04f412af4980f4750`：浏览器验收、图库/分类/防诈骗覆盖，及实测发现的手机底栏快速滚动修复。
- 本报告和最终截图等待动画稳定的测试补充另作证据提交；用 `git log -1 -- output/storefront-r5/SUMMARY.md` 定位。各提交均带指定 Co-Authored-By 尾注。
- 未改 Go、SQL、依赖锁文件、OrderFlow.tsx、主 checkout 或其他 worktree；未 push、merge、部署或读取生产密钥。

## 逐项交付

| 项目 | 状态 / 实现 SHA | 结果与边界 |
|---|---|---|
| G1 | **DEFERRED** / 上述两条原证据提交 | A 的内部取图不带 Host，且缓存键没有 Host；B（Go 上传时生成尺寸、media ?w=）归后端。本单没有实现 B。 |
| G2 | **LOCAL_VERIFIED** / a170583 | ProductAssurance 只读现有 buyer session 的 checkout-options；过滤 unavailable，逐页读完，付款方式只取明确返回的 payment_modes；不推断信用卡、不使用库存保留时间作为送达时间。展示已发布商家 shipping / returns / refunds 页面链接，不捏造退货承诺。 |
| G3 | **LOCAL_VERIFIED** / a170583、a3ccc2f | 复用 Host-scoped 公共目录和现有 ProductCard / Rail；取当前商品首个系列，排除当前商品、去重、最多 8 件；无系列/无其他商品/接口失败则隐藏。草稿和别店隔离由既有 Go 公共接口保证，不从后台目录补数据。 |
| G4 | **LOCAL_VERIFIED** / a170583、a3ccc2f | 列表页分类来自公共 collections，仅显示 product_count > 0；隐藏分类不回填，390px chips 换行，无横向溢出；保留选中态与预览参数。 |
| G5 | **LOCAL_VERIFIED** / a170583、a3ccc2f | /{locale}/legal/anti-fraud 三语通用防诈骗指南、页脚入口、BankTransfer 一行提示和链接。与既有五份需 owner 审定的政策模型分开，未改变其草稿门禁；OrderFlow 未改。 |
| G8 | **LOCAL_VERIFIED** / a170583、a3ccc2f | 原生 img + dialog 放大；上一张/下一张、方向键、Esc、关闭后焦点返回；无图不显示放大入口；打开期间锁定背景滚动。 |
| 视觉约束 | **LOCAL_VERIFIED** / a170583、069637c | 白底、商家橙色主题、44px 控件、三语 UI；TWD 全语言显示 NT$，整数无 .00，但真实分金额仍保留，绝不为视觉稿舍入。收藏数、虚拟账号均未实现。 |

G2 有意限制：匿名纯浏览不会为说明块新建会话；无会话时只显示已发布政策链接。接口失败、不完整分页、重复游标或超过 10 页时隐藏方式，购买功能保持可用；没有承诺只为预览新增后端匿名接口。缺少商家退货页面时隐藏该项，不写默认期限。

## 必跑门禁（最终源码实跑）

工作目录均为本工作区。表中退出码来自实际进程，不把构建成功当浏览器通过。

| 命令 | 退出码 | 证据 / 层级 |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | **0** | `test-node-final-rerun.log`；273 PASS，0 FAIL。R04 二进制测试单独 NOT_RUN，见下。 |
| `pnpm --filter storefront exec tsc --noEmit` | **0** | `tsc-final-rerun.log`（成功为空输出）。 |
| `bash scripts/dev/check-gates.sh` | **0** | `check-gates-accepted.log`；55 个模式；新增测试已进入 Git index 后检查。 |
| `bash scripts/dev/test-local.sh --browser-storefront` | **0** | `browser-storefront-accepted.log`；SFR01–09，真实 Go / 隔离 PG / production Next，本机合成 TLS edge。 |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | **0** | `browser-catalog-media-accepted.log`；20 cases，zh-TW/en × 桌面/390px；真实 Go/PG，MOCK IdP。 |
| `bash scripts/dev/test-local.sh --browser-storefront-publish` | **0** | `browser-storefront-publish-accepted.log`；23 cases，真实 Go/PG，MOCK IdP，无 owner-seeded publication/domain。 |

真实浏览器详细工件相对于 worktree：

- `output/playwright/storefront/20261002T084805.218244000`
- `output/playwright/catalog-media/20261002T084833.191939000`
- `output/playwright/storefront-publish-1598106694`

### 红→绿及补充验收

- `node --experimental-strip-types --test apps/storefront/tests/storefront-r5.test.mjs`：
  初次 **1**（G3/G4 两个断言失败，`red-g3-g4.log`）→ 最终 **0**（3 PASS，`green-g3-g4-accepted.log`）。
  单测验证排当前/去重/空系列与空分类；**不宣称纯函数测试证明数据库租户隔离**。
- 真实 SFR04 额外断言同系列不含当前、draft、retired；无系列隐藏。SFR03 额外检查 390px 分类与页面均无横向溢出。
- 全量 MOCK：
  `LC_SHOP_EVIDENCE="$PWD/output/storefront-r5/mock-sticky-green" LC_R5_SHOTS="$PWD/output/storefront-r5" node tests/storefront/shop-gate.mjs`
  → **0**，`browser-r5-sticky-green.log`：SF01–12 全过，再通过 R5 隐藏/空分类、撤回商品、foreign Host、明确 payment_modes/分页/503/不 bootstrap、三语指南和图库检查。
- 额外转账真实链：
  `bash scripts/dev/test-local.sh --browser-checkout-offline` → **0**，`browser-checkout-offline.log`；
  本地真实 Go/PG 合成订单、模拟身份，无 PSP/真实付款。三语中的 zh-TW/en、桌面/390px 检查转账提示入口；截图来自 `output/playwright/checkout-offline/20261002T083523.246847000`。
  此补充链在最后 ProductBuy 快速滚动修复前运行，BankTransfer 源码此后未改。
- 最终截图补捕：
  `LC_R5_ONLY=1 LC_SHOP_EVIDENCE="$PWD/output/storefront-r5/captures-settled" LC_R5_SHOTS="$PWD/output/storefront-r5" node tests/storefront/shop-gate.mjs`
  → **0**，`browser-captures-settled.log`。它仅跑 R5，输出 cases=0 指没有重复 SF01–12；不拿它代替全量门禁。

## 实测缺陷、失败记录与复核

1. 独立源码审查指出指南误混入商家政策草稿、退款标签混同退货、options 未翻页、图库可访问语义缺 role；均已修复并定向复核无剩余 P1。Humaux：`66f4db6f-f93e-4a2f-bbee-4d63af3e63ac`。
2. 全 MOCK 暴露既有 ProductBuy 的 IntersectionObserver 漏掉快速跳滚：actions 从视口下方直接到上方，两端均不相交，可能无回调。独立反例记录 bottom=-1221.75、scrollY=2320、无浏览器错误但无 sticky；`sticky-red.log` **1** 与 `sticky-red/sticky-position.json` 保留。
   改为 passive scroll/resize + rAF 每帧一次位置测量，初始化及清理齐全；全量 MOCK 转 **0**。原可见性/页脚遮挡断言全部保留，测试增加真正滚过 actions 的前置断言；没有改阈值。独立根因复核：`5673d07c-cdc3-4577-9397-625f1de30833`。
3. 早期 `browser-r5-mock.log` 的配送字段超时未单独证明根因；后续同一 SF05/SF11 全量通过。早期 `browser-r5-full-{final,confirmed}.log` 的底栏失败由上述反例闭环。
4. 初期新增防诈骗链接断言把页脚当前语言入口也算入重复链接，已精确限定政策导航且仍断言恰好一个；保留 `mock-diagnostic/` 的 DOM/错误/截图。早期类型收窄报错、启动/流式渲染等待失败也保留日志；最终类型检查及完整门禁通过。
5. impeccable audit-first 检查重复结构；复用现有卡片、横向产品轨道及原生 dialog，不引入新组件库。一次机械 detector 退出 **2**，仅既有 `sf-free__bar` 的 width transition 警告（`design-detect.json`），该行本单未改；不是全站“零问题”声明。
6. 独立视觉复核 disposition **ship（仅本次视觉范围）**：首轮 9 张首屏/图库、补充 12 张页底/安全/银行截图已检查，原 recapture 覆盖项解除，无新增视觉修改。英文长文继续向下滚动；不代表真实设备或生产行为复验。reviewer 已存 Humaux research「storefront-r5 独立视觉终审补充：原 recapture 覆盖项解除」。

## 截图

PNG 实际像素和 SHA256 见 `screenshots-current.json`；当前 UI 共 21 张，均在本目录：

- `product-{zh-TW,en}-{390x844,1586x992}.png`：商品页四组合。
- `products-{zh-TW,en}-{390x844,1586x992}.png`：列表页四组合。
- `gallery-en-desktop.png`：1586×992 原生放大。
- `{assurance,related,footer,anti-fraud}-{zh-TW,en}-390x844.png`：补齐页底状态。
- `bank-safety-{zh-TW,en}-{mobile,desktop}.png`：额外转账提示；mobile 390×844，已有银行门禁 desktop 1440×900（不替代上面的必需 1586×992 截图）。
- 页面使用合成商品/订单 fixture，不是生产客户数据。橙色截图来自隔离 fixture 的商家主题；不会强改已有商家的品牌色。英文 UI 保留商家输入的中文商品文案是既有契约。
- 原 `after-products-*.png` 是 **G1 破图诊断**，不得当当前 UI 验收图。历史 `COMMANDS.md` 也仅记录 G1 阶段。

## G1 历史性能证据与延期

同一原 fixture、390×844，原图三轮均 64,754 bytes / 4 张成功；A 三轮 0 张成功、优化请求 400；恢复原图后仍 64,754 bytes / 4 张成功。**0 bytes 是失败，不是压缩收益**。原始 `before-initial.json`、`after.json`、`before.json` 和旧证据提交保留。方案 B 性能收益、本次新 UI 图片性能对比与 LCP 非劣化均 **NOT_RUN / NOT_VERIFIED**，不得沿用旧 fixture 的结果冒充后端 B 通过。

## NOT_RUN / 局限 / 交接

- G1 B 全部后端实现及尺寸/跨 Host 缓存验收：DEFERRED 给后端单元。
- SANDBOX / LIVE 支付、生产部署、DNS/TLS/Caddy、真实买家或真实物流：NOT_RUN。
- Safari/WebKit、Firefox、实体手机、真实辅助技术：NOT_RUN；本轮 Chromium + 390px viewport，另有旧门禁 320px。
- zh-CN 文案/路由已功能与单测验证；zh-CN 独立截图 NOT_RUN（派单只要求 zh-TW/en）。
- R04 livekit runner：`COMMERCE_R04_LIVEKIT_BINARY` 未配置，NOT_RUN；不涉及本 UI 单元。
- 第二个**已发布**真实租户商品全生命周期隔离：本单未新增 Go fixture，NOT_RUN；有公共接口作用域源码核查、真实 draft/retired 排除和 MOCK foreign Host 拒绝，不扩大宣称覆盖。
- 同系列只取首个系列；公共接口不可用则隐藏。G2 无会话或方式接口不可用时不显示方式，是有意 fail-closed，不伪造默认选项。
- 主执行 Codex root（UI 写入）；运行时未暴露可核实的精确 root 模型/档位，不臆填。只读探索/源码复核：r5_gate_paths、r5_image_audit，explorer，gpt-6-luna / medium；独立视觉：r5_visual_review，explorer，gpt-6.1-sol / medium。均无递归委派、无文件写入，基线同本单。
- write_paths：本 worktree 的 `apps/storefront/**`、`tests/storefront/**`、`output/storefront-r5/**`；测试脚本在本 worktree 下生成隔离构建/浏览器证据。没有改 contracts / Go / SQL / OrderFlow。
- 进程、浏览器、合成 edge 和 PG 容器由脚本 finally/trap 回收；收尾未见属于 storefront-r5 的监听进程，未清理其他任务资源；失败证据全部保留。未删除用户数据或共享缓存。
- 代码增量索引已提交，ProductAssurance / relatedCards / CollectionChips / ProductGallery / BankTransfer / ProductBuy 已关联决策/修复记忆。Humaux 与本单画布随交付更新；Claude 仍负责独立集成终审与发布决策。
