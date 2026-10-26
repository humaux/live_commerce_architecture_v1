# SA-1 交付记录
- PR: #198 (base r3/integration, 无 auto-merge)
- 代码 head: b8d1a1d861de0133ebe3c2aaa56aa2335fa40cb9（复核轮修复）；PR head = 其后的本记录提交（只改本文件并回退 check_packet 生成的 packet-check.json，不改代码）
- RED run: 40323520531（PF16/PF17 行为红，PF18 缺入口红；见 output/settlement-returns/red.log）
- 首次实现 run 40328360114：g10 仅 TestStripeSL02Schema 红（ledger 6→7，已修）；其余 shard 当时未跑完，已被新推送取消
- 上一 head b73ca1cc5：Foundation 40359346072 success；Gates 40359345931 写本记录时仍 in_progress（--browser-product-media-v2 pending）
- 绿 run: 以 PR head（本记录提交）的 Foundation checks + Gates (GitHub runners) 为准；写本记录时未完成，run id 由 integrator 在 PR checks 读回（记录提交无法自含自身的 run id）
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

## 复核轮修复 · 2026-10-27（DeepSeek REQUEST_CHANGES + Opus APPROVE 的 P2；裁决 sa-cards-20261026.md）
- P1 tw_legal_opinion：live.go PlatformChecklist 5 码；0200 §9 原地 patch approve_stripe_live（v_full 16 码）与 set_stripe_platform_open（LIVE 需 5 码）；pfLiveChecklist 16 码。0200 前存的 15 码 approval 不再能开 LIVE，owner 需重新 attest。契约 §8 依据改为「律师意见或书面风险接受」（LAUNCH-REVIEW §6 owner 裁决 2026-10-26），gate 不变。
- P2 manifest：RecordSettlementLines 在分块前用 Go 校验整个 manifest（txn id 语法、id 唯一、^[A-Z]{3}$、abs(net)≤1e11），不合法 → ErrRejected、0 次 SQL。
- P2 契约 §6.7：guard_settlement_statement 不 patch，0150:203 整行比较已覆盖 returned_in_minor；0200 头注释同步引用。
- Opus P2：PF16 固定 T = W2 周一 12:00+08:00 + 干净覆盖前置；注错现象改为 settlement_return_immutable / settlement_changed；部署顺序写入契约 §9 与 deploy/README；stripe_live_schema_test 注释 five→seven。
- tasks.json：SA-2 acceptance 加 returned_in 列与 payout_returned ⇒ paid && payout_returned_at（integrator 批准的跨卡文字修改）。
- 新增测试：TestPlatformStripeLiveLegalOpinion（foundation，PG）；TestSettlementManifestLastChunkInvalidWritesNothing、TestPlatformChecklistRequiresTWLegalOpinion（stripeadmin 单测；后者放在 settlement_unit_test.go，因 live_test.go 不在 write_paths）。
- 本地：go build ./...、go vet（stripeadmin、tests/foundation）、gofmt -l、check_packet 通过（E1）。所有测试 NOT_RUN（仅 CI）。
