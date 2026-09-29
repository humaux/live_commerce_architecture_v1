<!--
File: docs/runbooks/merchant-onboarding.md
Purpose: 运维人员接入第一个真实商家的步骤（店铺、Stripe 账户、Meta Page、域名），并逐项标出只有 owner 能完成的前置条件和当前的工程缺口。
Runs as/in: 文档（运维人员在部署主机上以 root 执行其中命令；商家自己的操作在后台 UI 完成）。
Reads env / secrets: 无。命令通过 deploy/scripts/ops-admin.sh 提示输入运维输入（STRIPE_SECRET_KEY、STRIPE_ACCOUNT_ID、STRIPE_WEBHOOK_SECRET、META_PAGE_ACCESS_TOKEN），
  这些输入不写入任何文件；本文不含任何密钥值。
Used by: 运维/owner/集成者；docs/runbooks/deploy.md §6 链接到此。
Depends on: deploy/scripts/ops-admin.sh、deploy.md（栈已按 §3 部署且 healthy）、contracts/stripe-psp-v1.md §13、contracts/meta-claims-intake-v1.md §7。
Status: DESIGN。没有在真实商家上执行过；SANDBOX 步骤需要 owner 的 Stripe 测试账户（工程侧证据 NOT_RUN）。
Change rules: 命令必须与 deploy/scripts/ops-admin.sh 和 cmd/*-admin 的子命令保持一致；owner-only 项不得被写成“工程可以完成”。
-->
# 第一个商家接入手册

范围：R1（沙箱）。**任何生产部署、LIVE 支付、真实退款、向真实买家发送 Meta 消息，都必须有 owner 在聊天里的明确批准**（AGENTS.md）。
状态词汇：DESIGN / MODEL_ONLY / MOCK / SANDBOX / LIVE / NOT_RUN。

## 0. 只有 owner 能完成的前置条件（工程无法代办）

| # | 前置条件 | 谁 | 缺失时的后果 |
|---|---|---|---|
| O1 | 接受 Compose 作为生产形态的 ADR（架构.md §4.2，B8） | owner | 不得宣称“可生产” |
| O2 | OIDC IdP 选型并注册 client（回调 `https://<LC_ADMIN_HOST>/api/auth/callback`，B2） | owner | 商家无法登录 |
| O3 | 域名与 DNS：`LC_ADMIN_HOST`、`LC_STORE_HOST`、`LC_API_HOST`、`LC_HOOKS_HOST` 四个互不相同的名字指向部署主机 | owner | preflight P14 失败，无法签发证书 |
| O4 | Stripe：**测试**账户的 `sk_test_`/`rk_test_` key 和 `acct_...`（SANDBOX）；LIVE 激活是单独的 owner 决定，代码目前拒绝 LIVE | owner | Stripe 支付方式无法登记 |
| O5 | PAYUNi 商户资质与 LIVE 批准（B3）；NotifyURL 无接收端（B4） | owner | PAYUNi 只能 SANDBOX |
| O6 | Meta：App、App Review / Access Tier、Page 或 Instagram 资产、`commerce_meta_apps_json`（B6） | owner | 评论入口和私信都不可用 |
| O7 | 承运商合同、台湾超商物流面单（R2）；R1 只有人工发货（承运商 + 运单号，商家后台录入） | owner | 无自动面单 |
| O8 | 真实退款、真实买家消息、LIVE 支付的逐项批准 | owner | 一律不执行 |

## 1. 栈必须先就绪

按 deploy.md §1–§3 部署；`deploy/scripts/preflight.sh --online` 全部 PASS，`docker compose ps` 全部 healthy，
`deploy/scripts/watchdog.sh` 无 FAIL。`LC_ONBOARDING_ENABLED=1` 且 `LC_ONBOARDING_CURRENCIES` 含商家币种（例如 `TWD`）。

## 2. 店铺

1. 商家用 OIDC 登录后台，走 onboarding 创建租户和店铺。创建者自动获得 owner 权限和六个店铺权限：`live:read`、`live:manage`、`payments:refund`、`fulfillment:write`、`orders:export`、`integration:execute`（裁决 24，`0065`）。
   其他成员不会自动获得退款权限。
2. 运维记录 ID（只读查询，通过 `lc_psql`，见 lib.sh）：
   ```sh
   source deploy/scripts/lib.sh; lc_load_env "$LC_COMPOSE_ENV"
   lc_psql <<< "SELECT tenant_id, store_id, principal_id, created_at FROM ops.audit_events WHERE action = 'merchant.store_created' ORDER BY created_at DESC LIMIT 5;"
   ```
   后面所有 `ops-admin.sh` 命令都需要这三个 ID，`--principal` 必须是该租户的 owner 成员。

## 3. Stripe 账户（SANDBOX）

前提 O4，`LC_STRIPE_ENABLED=1`，`payments-sandbox` 在运行。完整步骤见 deploy.md §6.1；顺序是：
`stripe-admin register` → 在 Stripe 测试模式创建 webhook 端点（API 版本 `2026-08-26.dahlia`，8 个事件，含 4 个退款事件）→
`stripe-admin webhook` → `STRIPE_SANDBOX=1 stripe-admin qualify` → `stripe-admin method`。
商家不能自助登记 Stripe（把平台 key 绑到任意店铺会混同资金，contracts/stripe-psp-v1.md D1）。
**验收（SANDBOX，owner 执行）**：买家用 Stripe 测试卡 4242 完成支付，订单变为已支付；商家后台退款成功。工程侧此项为 NOT_RUN，MOCK 证据见 `tests/foundation` 的 SP/RF 系列。

## 4. Meta Page（评论入口 → 认领 → 私信）

前提 O6。步骤见 deploy.md §6.3：`commerce_meta_apps_json` → `COMMERCE_META_WEBHOOK_ENABLED=1` + `meta` profile →
`ops-admin.sh meta-admin route`（绑定 + 路由）→ `ops-admin.sh meta-admin page-token`（登记 Page token）→ 商家在后台把直播场次绑定到 Facebook 贴文/Instagram media（**目前不可用，见 G2**）→ owner 批准后启用 `claims` profile。

**G1 已关闭（R1 裁决 F2）**：`ops-admin.sh meta-admin route` 创建/复用店铺的 facebook/instagram 绑定并激活 webhook 路由（deploy.md §6.3 第 6 步），
需要 owner 提供资产所有权证据的 sha256（`--proof`）。顺序：route → page-token → 认领来源（G2）。

**G2 已关闭（R1 裁决 G2）**：部署默认 `COMMERCE_STUDIO_ENABLED=1`、`COMMERCE_CLAIMS_ENABLED=1`、`COMMERCE_STUDIO_MEDIA_ENABLED=0`，
`PUT .../live-sessions/{session}/claim-source` 已挂载（smoke S45 验证未带令牌时返回 401/403）。直播媒体（LiveKit 演练）仍不部署，Studio 页面不显示演练栏。详见 deploy.md §6.3 第 7 步。

## 5. 域名与证书

四个域名解析到部署主机后，Caddy 自动签发证书（首次演练可在 `caddy.env` 打开 staging CA，正式签发前注释掉）。
商家自有域名（品牌域名指向 storefront）不在 R1 范围：storefront 的 origin 由 `LC_STORE_HOST` 推导，Caddy 只服务这四个名字（不匹配的 Host 不转发）。

## 6. 接入后检查

- `deploy/scripts/watchdog.sh` 全部 PASS；`docker compose logs --since 10m claims-worker` 出现 `claims_worker_ready`（若启用）。
- 后台：店铺可见、Stripe 支付方式在结账页出现（沙箱）、订单/退款/发货页面可用。
- `deployments.log` 与 `ops-admin.log`（`$LC_STATE_DIR`）记录了本次操作者和子命令（不含参数值）。
