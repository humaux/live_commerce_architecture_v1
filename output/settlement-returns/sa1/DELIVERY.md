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
