# PR #73 复核 — restart-replay gate（HEAD d196e4beb）

**结论：APPROVE**（条件：CI `gate (--browser-identity)` / foundation 跑绿；复核时 run 38763342620 仍为 pending）

复核方式：只读静态复核，看 `git show d196e4beb`，跑了 `gofmt -l`（干净）、`GOTOOLCHAIN=go1.27.2 go vet -tags browser ./tests/foundation`（exit 0）、`scripts/dev/check-headers.sh`（OK）。Go 测试未在本地跑（NOT_RUN）。

## P0-1（jsonb 重复计数）— 已修复
- snapshot 文本里的 `strings.Count` 已删掉，换成用 owner 池对三张表分别直接计数，期望各为 1：
  - `inventory.warehouses`：按 tenant/store/name 过滤。name 带 t04Tag，每次唯一。
  - `ops.command_results`：按 `operation='inventory.warehouse.create' AND idempotency_key=$4` 过滤。operation 字符串与 internal/inventory/inventory.go:69 一致。command.go:88 写入的是原始 key，handler 原样传入 `Idempotency-Key` 头；`restart-replay:<tag>` 满足列上的 CHECK `^[A-Za-z0-9_.:-]{8,128}$`。
  - `ops.audit_events`：按 `action='inventory.warehouse.created'` 过滤。`action` 列存在（migrations/0001_foundation.sql:59），字符串与 inventory.go:74 一致。
- 不会把旧的审计行也算进来：pwIsolatedFixture 用独立 PG 容器，tenantA/storeA1 是随机 UUID；`f.seed` 只写 grant，不建仓库、不写审计；迁移里插入 audit_events 的语句不可能命中运行时才生成的租户。所以 audits=1 是准确的。
- snapshot 全量比对（两次 replay 前后）没变，仍能抓到改写 response/created_at 或重复写审计的情况。

## 日志扫描放进 t.Cleanup — 正确
- 扫描用的 Cleanup 在 `start()` 之前注册。按 LIFO 顺序，它会在每个进程的 kill/wait cleanup、以及 first.log/second.log 的文件关闭 cleanup（`browserLog`）**之后**执行，日志此时已完整落盘。
- 它在 PG 容器清理和 `runtime.Close` **之前**执行，不过这两个也不影响扫描：`f.runtime.Config()` 读的是已有的配置结构，不需要连接。
- 日志不存在（还没 start 就失败）时 `os.IsNotExist` 会 continue，其他读错误报 `t.Error`。Cleanup 里用的是 t.Error 而不是 t.Fatal，所以中途 Fatal 时其余 cleanup 照常执行。
- 新增扫描 runtime 角色的密码本身（`ConnConfig.Password`）。它是 48 位 hex 随机串（bcRole），不会误报。

## strict 200 — 正确
admin 事务路由用 `respond(w,200,…)`（上一轮已核实），replay 断言 `replay.Code == first.Code`，所以两次都必须是 200。

## 新缺陷
无。

## 仍为可选项（不阻塞）
- P2-1：start/stop/request 三个闭包与 account_process_test.go 几乎相同，可以抽成共享 helper。
- P3-2：每次运行都把 api 二进制留在 output/playwright/restart-replay-*/，不回收。
- P3-3：「同一个 PG」可以用 `pg_control_system().system_identifier` 显式断言。
- 小建议：owner 密码只作为完整的 `f.databaseURL` 被扫描，没有单独扫密码本身。子进程拿不到 owner DSN，风险低。

## 合并条件
CI foundation / `--browser-identity` gate 绿（附 run id）。不用再做静态复核。
