# SA-1 交付记录
- PR: #198 (base r3/integration, 无 auto-merge)
- head SHA: 8edc8b13bab0d580257486b51d7d1c69e400776b（CI 重跑后可能再变）
- RED run: 40323520531（PF16/PF17 行为红，PF18 缺入口红；见 output/settlement-returns/red.log）
- 首次实现 run 40328360114：g10 仅 TestStripeSL02Schema 红（ledger 6→7，已修）；其余 shard 当时未跑完，已被新推送取消
- 绿 run: 无 (NOT_RUN)。run 40337416721 在 GitHub runner 队列中，全部 job queued
## 变更
migrations/0200；stripeadmin/settlement.go；settlement/{service,export}.go；cmd/stripe-admin；stripe_runtime.go pin；ops-admin.sh/README/deploy 测试；foundation 测试适配；契约 §6.7–§10 + plan + tasks.json
## 风险
- 本地无 PG，SQL 仅 CI 验证；浏览器 card-payments 门可能因 SA-2 严格解析器拒绝新 key（未碰 SA-2 文件）
- g09 上 TestProductEditorMatrixLedgerEvidenceHistoricalPolicy 红，与本改动无关
## NOT_RUN
所有测试（仅 go build/vet/gofmt/check_packet 本地过，E1）；PR 绿未取得

## Integrator 接手修复 · 2026-10-27（CI run 40351516453，gate --browser-card-payments 中 STL1 得 0 行）
- 根因：基线 UI 的 `parseSettlementStatement` 只认固定字段，遇到额外字段就判为不可用。SA-1 新增了 3 个字段（`returned_in_minor`、`payout_returned`、`payout_returned_at`），后台列表因此显示「Settlements are unavailable」。这也意味着生产上如果后端先于前端发布，同样会出问题。
- 修法（先扩展、后收紧）：3 个字段在旧 parser 中按可选处理并校验类型；净额校验加上 `returned_in_minor`（缺省为 0，对应 0200 新的 CHECK）。SA-2 rebase 后会把这 3 个字段改为必填并负责展示。integrator 已批准把该文件临时加入 SA-1 的 write_paths。
- 证据：playwright 的 error-context 位于 artifact gate-browser-card-payments（id 12665333591）。修复后的结果以新一轮 CI 为准，本地没有运行测试。
