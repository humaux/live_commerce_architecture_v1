# Unit W5-02B — 顾客 CSV 导入（含最小导入批次框架，后端）

状态：DRAFT（起草人 Claude Opus 子代理 2026-10-06，待 integrator 审核/冻结）。Base `r3/integration` `bb71f966`。
迁移号占位 **0140**。覆盖 IMPLEMENTATION-PLAN W5-01B（框架，**并入本单元的最小版**）+ W5-02B（M22 #6）。
worktree `.worktrees/w5-02b-customer-import`（branch `unit/w5-02b-customer-import`）。UI 在 W5-U1。
先读：PREAMBLE → AGENTS.md → PROCESS.md → 本文件；合同只读 `contracts/customers-billing-v1.md` §2、§3.1、§6、§7、§12；
`docs/delivery/units/w3-01b-tracking-backfill.md`（CSV 预览/提交/幂等先例）；`w6-01b-customer-tags-notes.md`。

## Integrator 裁决（覆盖正文）
- 计划 W5-01B 的「通用导入框架 + 商品 CSV 迁移到框架」**不做**（YAGNI：现有商品导入能用）；只建顾客与订单导入共用的两张表（批次、外部 ID）。
- **营销同意一律「未知」**：导入不写任何 `customers.consent_events`；CSV 里的「同意」列被忽略并在预览里标 `consent_ignored`。不得补造同意。
- 导入顾客是独立的 `buyer.owners` 行，**不与将来下单的买家自动合并**（身份解析需 verified-contact 合同，偏差 B24 延后）。
- fixture 一律合成数据；owner 的 SHOPLINE 样本只用于定列名映射，**不进仓库、不进日志、不进 Agent 上下文**（若含 PII 先由 owner 去识别化）。
- 必须在 W6-01B 合并后开工（两者都改 `identity.read_merchant_customers`）。

## 目标 / owner 流程
从 SHOPLINE 后台导出顾客 CSV → 上传 → 映射列（系统按 SHOPLINE 列名自动猜，商家可改）→ 预览（新增 N、更新 M、失败 K）→ 提交 →
客户列表出现这些顾客（标记「匯入」），之后 W5-03B 再导入他们的历史订单。

## 关键事实（bb71f966）
- 客户列表只显示「≥1 订单或已绑定 bundle」的 owner（`read_merchant_customers` 注释，0078:960）→ 导入的 owner 不改此谓词就看不到。
- CSV 先例：`internal/merchanttools/csvfile.go`（2 MiB、UTF-8/BOM、RFC 4180、`guardCell/unguardCell`）、`csvimport.go`（预览=同代码整笔回滚，提交=`command.Run` 文件哈希幂等，逐行 savepoint）。
- erasure `customers.erase_owner`（0078:460）。

## 范围
1. 表（schema `migrationimport`）：`batches(tenant_id, store_id, id, kind CHECK IN ('customers','orders'), file_sha256, mapping jsonb ≤ 2 KiB, rows_total, applied, updated, failed,
   results jsonb（每行 `{row, outcome, code?, external_id}`，无 PII）, principal_id, created_at, UNIQUE(tenant_id,store_id,kind,file_sha256))`；
   `external_ids(tenant_id, store_id, kind, external_id text 1..64, internal_id uuid, PRIMARY KEY(tenant_id,store_id,kind,external_id))`。批次保留 90 天。
2. 表 `customers.import_profiles(tenant_id, store_id, owner_id PK 部分, display_name 1..80, phone_e164 NULL, email NULL（规范化小写、≤254）, source CHECK='shopline_csv',
   imported_at, updated_at)`。
3. 字段：`external_id`（必填，SHOPLINE 顾客 ID）、`name`、`phone`（台湾手机规范化成 E.164，非法 → 行失败 `invalid_phone`）、`email`、`notes`（**忽略**，见 OPEN-2）。
   同一 external_id 再导入 → 更新 profile（`updated`），不新建 owner。
4. 预览 `POST …/imports/customers/preview`（`text/csv`，`?mapping=` JSON）/ 提交 `…/commit`（`Idempotency-Key` + `expected_apply_rows`，漂移 → `preview_stale`）/
   结果 `GET …/imports/{batch_id}/results.csv?only=failed`。上限 2 MiB、5000 行、60 s `WithScopeBudget`。权限 **`customers:privacy`**（写入 PII，owner 级）。审计 `customers.imported`（计数，无 PII）。
5. `read_merchant_customers`：谓词加「或存在 `import_profiles`」；显示名/电话末 3 位在无订单时取 import_profiles；行加 `imported: bool`。
6. `erase_owner`：同事务清除 `import_profiles` 行与该 owner 的 `external_ids` 行。

## Non-goals
商品导入迁移到框架、身份合并、导入同意、导入标签/备注、导入积分/会员等级、Big5 解码、异步大文件。

## 合同修改
新增 `contracts/migration-import-v1.md`（integrator 冻结：两表、CSV 规则、映射格式、上限、同意规则、身份不合并）；
`customers-billing-v1.md` Amendment：列表谓词、erasure 覆盖 import_profiles。

## SQL / 迁移 `0140_customer_import.sql`
- 新 schema `migrationimport`（REVOKE ALL FROM PUBLIC）；所有新表 ENABLE+FORCE RLS，**无登录角色直接 GRANT**。
- 定义者 `migrationimport.import_customer_row(...)`、`migrationimport.record_batch(...)`：owner `commerce_privacy_writer`（buyer.owners 插入需要的权限若不在该角色，
  实现者在 DELIVERY 提出最小 GRANT，由 integrator 裁决），SECURITY DEFINER，`search_path=pg_catalog`，`customers:privacy`，EXECUTE 仅 `commerce_runtime`。
- `CREATE OR REPLACE identity.read_merchant_customers`（W6-01B 之后的签名）与 `customers.erase_owner`：ACL 不变。
- ACL 钉子：`customers_billing_schema_test.go`、`customers_billing_consent_test.go`（导入后 consent 为未知；erasure 覆盖）、`buyer_registration_test.go`（owner 插入路径不被滥用）、
  `worker_authority_split_test.go` WAS02。

## 写入路径
`internal/migrationimport/{batch.go,customers.go}`（新）、`internal/customers/read.go`（只加 `imported` 字段解码）、`internal/httpapi/imports.go`（新）、
`migrations/0140_customer_import.sql`、`tests/foundation/customer_import_test.go`。

## 测试（合成 fixture `tests/foundation/testdata/shopline_customers_synthetic.csv`）
| # | 用例 | 期望 |
|---|---|---|
| CI01 | 3 行合法预览 → 提交 | 3 owner + 3 profile + 3 external_ids；列表可见 `imported:true` |
| CI02 | 同档重放 / 同档不同 N | 原结果 / 409 |
| CI03 | 同 external_id 新档 | `updated`，不新建 owner |
| CI04 | 「同意行銷」列为「是」 | 0 条 consent_events，预览 `consent_ignored` |
| CI05 | 非法电话、超长名、公式注入格 | 行失败 / 行失败 / 存储前 unguard、结果档 guard |
| CI06 | 无 `customers:privacy`（admin 以下） | 403 |
| CI07 | erasure 导入顾客 | profile 与 external_id 删除，列表消失 |
| CI08 | 5001 行、2 MiB+1、Big5 | `too_many_rows`、413、`encoding_not_utf8` |
| CI09 | 结果档/日志/审计扫描 | 无姓名、电话、email |

## 门禁
`bash scripts/dev/test-focused.sh '^TestCustomerImport'`；新模式 `bash scripts/dev/test-local.sh --migration-import`（integrator 加）；回归 `--browser-customers-billing`；
`release-gate.sh --strict --only G07`；`check-gates.sh`。

## 证据 / 角色 / 依赖
REAL_PG/MOCK（合成数据）。DeepSeek；K3（写「同意不被补造」反例）；Claude 复核 PII。依赖 **W6-01B 合并**。W5-03B 依赖本单元。

## OPEN
- CI-OPEN-1 owner 需提供去识别化 SHOPLINE 顾客导出**表头**（只要列名即可）以定自动映射；没有 → 映射全手动。
- CI-OPEN-2 SHOPLINE 备注列：**推荐**本单元忽略；W6-01B 合并后若 owner 需要，另开 S 单元导入为备注。
