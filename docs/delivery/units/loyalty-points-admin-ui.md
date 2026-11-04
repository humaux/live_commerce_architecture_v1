# Unit LP-2 — 會員積分後台介面（會員列表/詳情、加扣點、手工單贈點、訂單撤回）

状态：DRAFT（contract writer 2026-11-05，待 integrator 冻结）。Role `ui_worker`；worktree `.worktrees/loyalty-points-admin-ui`（branch `unit/loyalty-points-admin-ui`）。不得再委派。无迁移。
合同：`contracts/loyalty-points-v1.md`（LP7、LP9、LP13、LP15、LP17、§2、§3、§5 LPU01–LPU06）。

## 依赖
- **LP-1 已合并且路由冻结**（不得对未冻结接口并行实现）。
- **PARITY-MANUAL-ORDER 已合并**：它是手工單页面（`apps/admin/components/ManualOrder.tsx`、`apps/admin/app/[locale]/orders/new/**`）的 owner；本单元在其合并后的版本上只加「會員」与「本單贈送積分」两个字段。若该卡未合并 → BLOCKED，不抢改其文件。
- 导航位置与视觉：按已签署 frame / Figma；若无 frame → 先由 integrator 给出位置（合同 Q5），本单元不自造版式。

## 范围
1. 會員列表 `apps/admin/app/[locale]/members/page.tsx`：搜索（名字或联系备注）、积分余额、游标「載入更多」、「新增會員」。
2. 會員详情 `apps/admin/app/[locale]/members/[member]/page.tsx`：资料、积分明细（时间、±点数、原因、订单链接、备注、员工）、「加點 / 扣點」对话框（整数、必填原因）、编辑、匯出、清除（确认框说明「點數保留、姓名移除」）。
3. 手工單：可选会员选择器（搜索 + 内联新增）与「本單贈送積分」整数，默认空；空则不发送 `member_award` 键；选会员可预填订购人姓名（可改）。
4. 订单详情：「本單積分」面板 + 「撤回本單積分」（一键 = 剩余点数，可改小，必填原因）。
5. BFF `apps/admin/app/api/stores/[store]/members/**`，客户端 `apps/admin/lib/members-client.ts`；Idempotency-Key 由调用方生成、重试复用。
6. 三语文案（zh-TW / zh-CN / en）；按权限（`customers:read` / `customers:points` / `customers:privacy`）显示，而非按角色名。

## write_paths
`apps/admin/app/[locale]/members/**`、`apps/admin/app/api/stores/[store]/members/**`、`apps/admin/lib/members-client.ts`、`apps/admin/components/members/**`、
`apps/admin/components/ManualOrder.tsx`（仅新增两字段）、`apps/admin/app/[locale]/orders/[order]/page.tsx`（仅新增积分面板）、
admin 文案文件中 `members.*` 键、`tests/admin/members.spec.ts`、`tests/admin/members.test.ts`、`output/loyalty-points-admin-ui/**`。
**integrator only**：pnpm 锁文件（不应新增依赖）、shell 导航注册表若属共享文件、BFF route table / contractdrift baseline。

## 不做
买家端任何页面、通知、积分规则/自动计算、到期、折抵、赠品、会员合并、与网站客户（`buyer.owners`）关联、CSV、报表/广告入口、在 URL/日志/console/analytics 中放姓名或联系备注、`for-buyer`（直播代建）发积分、新依赖。

## 门禁（先红后绿；每项在 `output/loyalty-points-admin-ui/` 留「注错→红→修→绿」日志）
合同 §5 LPU01–LPU06：真实浏览器、真实点击、REAL_PG 种子店、375 px + 桌面、三语。另：TypeScript strict 编译、lint、受影响的既有端到端模式（手工單、订单详情、导航 click-sweep）全部重跑，因为改了路由与页面形状。
交付记录同 LP-1；作者 PASS = E0，独立复核由 test_worker。
