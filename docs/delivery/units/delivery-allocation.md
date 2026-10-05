# Unit brief: delivery-allocation（P0：新店配送服务没有仓库分配，买家结账无配送选项）

## 背景与证据（integrator 2026-10-05 代码复核）
- 结账配送选项查询 `internal/checkout/options.go:190` 与 `lockAllocation`（`internal/checkout/checkout.go:707`）强依赖 `fulfillment.allocation_heads/allocation_versions`。
- 唯一写入入口 `fulfillment.SetAllocation`（`internal/fulfillment/allocation.go:34`）在仓库里只有测试调用（`tests/foundation/delivery_allocation_test.go`），迁移里没有任何默认插入，管理后台/BFF/CLI 都不调用它。
- 所有浏览器门禁都用 Go 夹具直接造 allocation，所以没暴露。结果：商家在「設定」里启用配送服务后，买家结账可能没有任何配送选项。审计来源 `output/arch-conformance/m03-m12-m15.md` P0-1。

## 目标
商家只在后台启用一个配送服务（宅配 / 超商 / 自取…），不需要懂「仓库分配」，买家结账就能看到并选到它。

## 范围
- 后端（DeepSeek）：在启用/更新配送服务的**同一事务**里，为该服务确保存在指向店铺默认仓库的 allocation（店铺只有一个启用仓库时自动选它；多个仓库时按创建顺序取第一个并在响应里返回，供界面后续显示）。必须幂等、遵守现有 CAS 版本（expected_version / expected_service_version）语义、RLS 与 SECURITY DEFINER 规范（固定 search_path）。服务停用不删 allocation（保留历史版本），只是不再出现在选项里。
- 迁移：一次性 backfill——对「已启用配送服务但缺 allocation」的店铺补建（前向、带校验和、可重复执行无副作用）。编号由 integrator 分配，草拟用 0117。
- 不在本单元：多仓库优先级编辑界面（之后由 Codex 在设定页做）；运费规则改动。

## 不变量（contracts/invariants.json）
关键写入与任务同事务；租户/店铺范围只取自服务端认证；无仓库或仓库停用时返回明确错误码（不静默成功）；并发两个请求不得产生两个 head。

## 测试（先红后绿）
1. 新增 foundation 测试：走**商家真实 HTTP 路径**（与后台设定页相同的 BFF/Go 路由）启用一个配送服务 → 不经任何 Go 夹具 → 买家报价/结账选项里出现该服务。先在当前代码上证明失败（红），再修好（绿）。
2. 并发启用同一服务 2 次 → 只有 1 个 allocation head（-race）。
3. backfill 迁移测试：造「有服务无 allocation」→ 迁移后补齐；再跑一次无变化。
4. 找出现有浏览器门禁里用 Go 夹具造 allocation 的地方，至少一个改为走真实设定路径（不删除旧断言）。

## 门禁
`go test -race ./internal/fulfillment/... ./internal/checkout/... ./tests/foundation/ -run 'Allocation|DeliveryOption|Checkout'`；`bash scripts/dev/check-gates.sh`；相关浏览器模式（在 scripts/dev/test-local.sh 里找设定+结账模式，例如 --browser-checkout-offline、--browser-home-cod），报告真实命令与退出码。

## 交付
实现、测试命令、退出码、证据文件、风险、NOT_RUN 清单；只改本单元路径：internal/fulfillment/**、internal/checkout/**（仅必要处）、internal/httpapi/**（配送服务路由）、migrations/0117_*、tests/foundation/*allocation*、tests/** 中被改为真实路径的门禁文件。
