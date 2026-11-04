# Unit LP-2 — 會員積分後台介面（會員列表/詳情、加扣點、手工單贈點、訂單扣回）

状态：DRAFT round 2（contract writer 2026-11-05，review 549 round 1 已改，待 integrator 冻结）。Role `ui_worker`；worktree `.worktrees/loyalty-points-admin-ui`（branch `unit/loyalty-points-admin-ui`）。不得再委派。无迁移。
合同：`contracts/loyalty-points-v1.md`（LP7、LP10、LP13、LP15、LP17、LP18、LP19、§2、§3、§5 LPU01–LPU07）。

## 依赖
- **LP-1 已合并且路由冻结**（不得对未冻结接口并行实现）。
- **PARITY-MANUAL-ORDER 已合并**：它是手工單页面（`apps/admin/components/ManualOrder.tsx`、`apps/admin/app/[locale]/orders/new/**`）的 owner；本单元在其合并后的版本上只加「會員」与「本單贈送積分」两个字段。若该卡未合并 → BLOCKED，不抢改其文件。
- 导航位置与视觉：按已签署 frame / Figma；若无 frame → 先由 integrator 给出位置（合同 Q5），本单元不自造版式。

## 范围
1. 會員列表 `apps/admin/app/[locale]/members/page.tsx`：搜索（名字或联系备注，经 `POST /members/search` body 发送，**不进 URL**）、积分余额、游标「載入更多」、「新增會員」。
2. 會員详情 `apps/admin/app/[locale]/members/[member]/page.tsx`：资料、积分明细（时间、±点数、原因、订单链接、备注、员工）、「加點 / 扣點」对话框（整数、必填原因、可选订单引用）、编辑、匯出、清除（确认框说明「點數保留、姓名移除」）。
3. 手工單：可选会员选择器（搜索 + 内联新增）与「本單贈送積分」整数，默认空；空则不发送 `member_award` 键；选会员可预填订购人姓名（可改）；回应 `member_award.state='not_awarded'` 时在已建订单旁显示「積分未贈送」及原因。
4. 订单详情：「本單積分」面板（`GET members/entries?order_id=`）+ 「扣回本單積分」= 预填的手动扣点（会员 = 该单赠点会员、点数 = −赠点、可改、订单引用 = 本单、必填原因）；无上限逻辑，余额不足时显示「積分不足」。
5. BFF `apps/admin/app/api/stores/[store]/members/**`，客户端 `apps/admin/lib/members-client.ts`；Idempotency-Key 由调用方生成、重试复用；BFF 不记录 body。
6. 三语文案（zh-TW / zh-CN / en）；按权限（`customers:read` / `customers:points` / `customers:privacy`）显示，而非按角色名。

## write_paths
`apps/admin/app/[locale]/members/**`、`apps/admin/app/api/stores/[store]/members/**`、`apps/admin/lib/members-client.ts`、`apps/admin/components/members/**`、
`apps/admin/components/ManualOrder.tsx`（仅新增两字段与 not_awarded 提示）、`apps/admin/app/[locale]/orders/[order]/page.tsx`（仅新增积分面板）、
admin 文案文件中 `members.*` 键、`tests/admin/members.spec.ts`、`tests/admin/members.test.ts`、`output/loyalty-points-admin-ui/**`。
**integrator only**：pnpm 锁文件（不应新增依赖）、shell 导航注册表若属共享文件、BFF route table / contractdrift baseline。

## 不做
买家端任何页面、通知、积分规则/自动计算、到期、折抵、赠品、独立撤回/上限逻辑、会员合并、与网站客户（`buyer.owners`）关联、CSV、报表/广告入口、在 URL/日志/console/analytics 中放姓名、联系备注或搜索词、`for-buyer`（直播代建）发积分、新依赖。

## 门禁（先红后绿；每项在 `output/loyalty-points-admin-ui/` 留「注错→红→修→绿」日志）
合同 §5 LPU01–LPU07：Playwright 真实点击（AGENTS.md「UI 验收」：click/fill/keyboard，禁止 `force`、dispatchEvent、page.evaluate 改 DOM 代替点击），断言用户可见结果且刷新后仍在；REAL_PG 种子店、375 px + 桌面、三语；交付「点击台账」（页面、控件、操作、期望、实际、通过/失败）。另：TypeScript strict 编译、lint、受影响的既有端到端模式（手工單、订单详情、导航 click-sweep）全部重跑，因为改了路由与页面形状。
交付记录同 LP-1；作者 PASS = E0，独立复核由 test_worker。
